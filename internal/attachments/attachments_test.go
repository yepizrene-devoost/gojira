package attachments

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

type fakeUploader struct {
	settings    jira.AttachmentSettings
	settingsErr error
	failOn      string
	uploads     []string
	contents    []string
	issue       string
	onSettings  func()
	onAdd       func(filename string)
}

func (f *fakeUploader) GetAttachmentSettings() (jira.AttachmentSettings, error) {
	if f.onSettings != nil {
		f.onSettings()
	}
	return f.settings, f.settingsErr
}

func (f *fakeUploader) AddAttachment(issueKey, filename string, content io.Reader) ([]jira.Attachment, error) {
	f.issue = issueKey
	if filename == f.failOn {
		return nil, &jira.MutationError{State: jira.MutationUnknown, Err: errors.New("transport")}
	}
	if f.onAdd != nil {
		f.onAdd(filename)
	}
	body, err := io.ReadAll(content)
	if err != nil {
		// A real client fails the request when the body stream errors.
		return nil, &jira.MutationError{State: jira.MutationUnknown, Err: err}
	}
	f.uploads = append(f.uploads, filename)
	f.contents = append(f.contents, string(body))
	return []jira.Attachment{{ID: "1", Filename: filename, Size: int64(len(body))}}, nil
}

func enabled(limit int64) *fakeUploader {
	return &fakeUploader{settings: jira.AttachmentSettings{Enabled: true, UploadLimit: limit}}
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUploadSendsEachFileInOrderWithItsBaseName(t *testing.T) {
	dir := t.TempDir()
	a, b := writeFile(t, dir, "a.txt", "alpha"), writeFile(t, dir, "b.txt", "beta")
	fake := enabled(1000)

	results, err := Upload(fake, "ARA-1", []string{a, b})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if strings.Join(fake.uploads, ",") != "a.txt,b.txt" || strings.Join(fake.contents, ",") != "alpha,beta" || fake.issue != "ARA-1" {
		t.Fatalf("uploads=%v contents=%v issue=%q", fake.uploads, fake.contents, fake.issue)
	}
	if len(results) != 2 || results[0].Path != a || results[1].Attachments[0].Filename != "b.txt" {
		t.Fatalf("results = %+v", results)
	}
}

func TestUploadRefusesBadPathsBeforeUploadingAnything(t *testing.T) {
	dir := t.TempDir()
	good := writeFile(t, dir, "ok.txt", "x")
	cases := map[string]string{
		"missing":   filepath.Join(dir, "nope.txt"),
		"directory": dir,
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			fake := enabled(1000)
			_, err := Upload(fake, "ARA-1", []string{good, bad})
			if err == nil || !strings.Contains(err.Error(), bad) || len(fake.uploads) != 0 {
				t.Fatalf("err=%v uploads=%v, want an error naming %q and no upload", err, fake.uploads, bad)
			}
		})
	}
}

func TestUploadRefusesFileOverTheInstanceLimit(t *testing.T) {
	dir := t.TempDir()
	big := writeFile(t, dir, "big.bin", "0123456789")
	fake := enabled(5)
	_, err := Upload(fake, "ARA-1", []string{big})
	if err == nil || !strings.Contains(err.Error(), "big.bin") || len(fake.uploads) != 0 {
		t.Fatalf("err=%v uploads=%v", err, fake.uploads)
	}
}

func TestUploadRefusesWhenAttachmentsAreDisabled(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeUploader{settings: jira.AttachmentSettings{Enabled: false, UploadLimit: 1000}}
	_, err := Upload(fake, "ARA-1", []string{writeFile(t, dir, "a.txt", "x")})
	if err == nil || len(fake.uploads) != 0 {
		t.Fatalf("err=%v uploads=%v", err, fake.uploads)
	}
}

func TestUploadUploadsNothingWhenSettingsCannotBeRead(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeUploader{settingsErr: errors.New("boom")}
	_, err := Upload(fake, "ARA-1", []string{writeFile(t, dir, "a.txt", "x")})
	if err == nil || len(fake.uploads) != 0 {
		t.Fatalf("err=%v uploads=%v", err, fake.uploads)
	}
}

func TestUploadStopsAtFirstFailureAndKeepsEarlierResults(t *testing.T) {
	dir := t.TempDir()
	a, b, c := writeFile(t, dir, "a.txt", "1"), writeFile(t, dir, "b.txt", "2"), writeFile(t, dir, "c.txt", "3")
	fake := enabled(1000)
	fake.failOn = "b.txt"

	results, err := Upload(fake, "ARA-1", []string{a, b, c})
	if err == nil || jira.MutationStateOf(err) != jira.MutationUnknown {
		t.Fatalf("err=%v state=%v", err, jira.MutationStateOf(err))
	}
	if len(results) != 1 || results[0].Path != a || strings.Join(fake.uploads, ",") != "a.txt" {
		t.Fatalf("results=%+v uploads=%v", results, fake.uploads)
	}
}

func TestUploadExpandsLeadingTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeFile(t, home, "note.txt", "hi")
	fake := enabled(1000)

	if _, err := Upload(fake, "ARA-1", []string{"~/note.txt"}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if strings.Join(fake.uploads, ",") != "note.txt" {
		t.Fatalf("uploads = %v", fake.uploads)
	}
}

func TestUploadRequiresAtLeastOnePath(t *testing.T) {
	fake := enabled(1000)
	if _, err := Upload(fake, "ARA-1", nil); err == nil {
		t.Fatal("want an error for no paths")
	}
}

// trackOpens records every handle the package opens so tests can prove each
// one is closed on every path.
func trackOpens(t *testing.T) func() []*os.File {
	t.Helper()
	var mu sync.Mutex
	var opened []*os.File
	orig := openFile
	openFile = func(name string) (*os.File, error) {
		h, err := orig(name)
		if err == nil {
			mu.Lock()
			opened = append(opened, h)
			mu.Unlock()
		}
		return h, err
	}
	t.Cleanup(func() { openFile = orig })
	return func() []*os.File { mu.Lock(); defer mu.Unlock(); return opened }
}

func assertAllClosed(t *testing.T, opened []*os.File) {
	t.Helper()
	for _, h := range opened {
		if _, err := h.Stat(); err == nil {
			t.Errorf("handle %s left open", h.Name())
		}
	}
}

func TestUploadSendsTheValidatedHandleWhenThePathIsReplaced(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.txt", "validated")
	fake := enabled(1000)
	fake.onSettings = func() {
		other := writeFile(t, dir, "swap.txt", "attacker-content")
		if err := os.Rename(other, path); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Upload(fake, "ARA-1", []string{path}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if len(fake.contents) != 1 || fake.contents[0] != "validated" {
		t.Fatalf("contents = %v, want the validated file's bytes", fake.contents)
	}
}

func TestUploadFailsInsteadOfTruncatingWhenTheFileGrows(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.txt", "abc")
	fake := enabled(1000)
	fake.onSettings = func() {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.WriteString("def")
		_ = f.Close()
	}

	_, err := Upload(fake, "ARA-1", []string{path})
	if err == nil || len(fake.uploads) != 0 {
		t.Fatalf("err=%v uploads=%v, want a failure and no completed upload", err, fake.uploads)
	}
	if got := jira.MutationStateOf(err); got != jira.MutationUnknown {
		t.Fatalf("state = %v, want unknown after dispatch", got)
	}
}

func TestUploadFailsInsteadOfTruncatingWhenTheFileShrinks(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.txt", "abcdef")
	fake := enabled(1000)
	fake.onSettings = func() {
		if err := os.Truncate(path, 2); err != nil {
			t.Fatal(err)
		}
	}

	_, err := Upload(fake, "ARA-1", []string{path})
	if err == nil || len(fake.uploads) != 0 {
		t.Fatalf("err=%v uploads=%v, want a failure and no completed upload", err, fake.uploads)
	}
	if got := jira.MutationStateOf(err); got != jira.MutationUnknown {
		t.Fatalf("state = %v, want unknown after dispatch", got)
	}
}

func TestUploadNeverStreamsMoreThanTheValidatedSize(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.txt", "abc")
	fake := enabled(3)
	var seen []byte
	fake.onAdd = func(string) {
		f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		_, _ = f.WriteString(strings.Repeat("x", 4096))
		_ = f.Close()
	}
	var up Uploader = &captureUploader{fakeUploader: fake, seen: &seen}

	_, err := Upload(up, "ARA-1", []string{path})
	if err == nil || len(seen) > 3 {
		t.Fatalf("err=%v streamed=%d bytes, want a failure after at most 3", err, len(seen))
	}
}

// captureUploader keeps the bytes the client managed to read.
type captureUploader struct {
	*fakeUploader
	seen *[]byte
}

func (c *captureUploader) AddAttachment(issueKey, filename string, content io.Reader) ([]jira.Attachment, error) {
	if c.onAdd != nil {
		c.onAdd(filename)
	}
	body, err := io.ReadAll(content)
	*c.seen = body
	if err != nil {
		return nil, &jira.MutationError{State: jira.MutationUnknown, Err: err}
	}
	return nil, nil
}

func TestUploadKeepsEarlierResultsAndStopsWhenALaterFileChanges(t *testing.T) {
	dir := t.TempDir()
	a, b, c := writeFile(t, dir, "a.txt", "1"), writeFile(t, dir, "b.txt", "22"), writeFile(t, dir, "c.txt", "3")
	fake := enabled(1000)
	fake.onAdd = func(name string) {
		if name == "b.txt" {
			f, _ := os.OpenFile(b, os.O_APPEND|os.O_WRONLY, 0)
			_, _ = f.WriteString("more")
			_ = f.Close()
		}
	}

	results, err := Upload(fake, "ARA-1", []string{a, b, c})
	if err == nil || len(results) != 1 || results[0].Path != a || strings.Join(fake.uploads, ",") != "a.txt" {
		t.Fatalf("err=%v results=%+v uploads=%v", err, results, fake.uploads)
	}
	if got := jira.MutationStateOf(err); got != jira.MutationUnknown {
		t.Fatalf("state = %v, want unknown", got)
	}
}

func TestUploadReportsAChangeEvenWhenTheClientIgnoresTheReadError(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.txt", "abc")
	fake := enabled(1000)
	fake.onSettings = func() {
		f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		_, _ = f.WriteString("def")
		_ = f.Close()
	}
	up := &lenientUploader{fakeUploader: fake}

	_, err := Upload(up, "ARA-1", []string{path})
	if err == nil {
		t.Fatal("want an error: Jira accepted content that did not match the validated file")
	}
	if got := jira.MutationStateOf(err); got != jira.MutationApplied {
		t.Fatalf("state = %v, want applied (client reported success)", got)
	}
}

func TestUploadRetainsNonEOFReadFailureWhenUploaderIgnoresIt(t *testing.T) {
	for _, state := range []jira.MutationState{"", jira.MutationUnknown, jira.MutationNotApplied, jira.MutationApplied} {
		t.Run(string(state), func(t *testing.T) {
			opened := trackOpens(t)
			path := writeFile(t, t.TempDir(), "a.txt", "abc")
			uploader := &readFailureUploader{fakeUploader: enabled(100), opened: opened, state: state}
			results, err := Upload(uploader, "ARA-1", []string{path})
			want := state
			if want == "" {
				want = jira.MutationApplied
			}
			if err == nil || jira.MutationStateOf(err) != want {
				t.Fatalf("err=%v state=%v, want %v", err, jira.MutationStateOf(err), want)
			}
			if state == "" && !errors.Is(err, os.ErrClosed) {
				t.Fatalf("err=%v, want original read failure", err)
			}
			if (len(results) == 1) != (want == jira.MutationApplied) {
				t.Fatalf("results=%+v state=%v", results, want)
			}
		})
	}
}

type readFailureUploader struct {
	*fakeUploader
	opened func() []*os.File
	state  jira.MutationState
}

func (u *readFailureUploader) AddAttachment(_, _ string, content io.Reader) ([]jira.Attachment, error) {
	_ = u.opened()[0].Close()
	_, _ = io.Copy(io.Discard, content)
	if u.state != "" {
		return nil, &jira.MutationError{State: u.state, Err: errors.New("client failure")}
	}
	return nil, nil
}

// lenientUploader swallows body read errors and reports success.
type lenientUploader struct{ *fakeUploader }

func (l *lenientUploader) AddAttachment(_, _ string, content io.Reader) ([]jira.Attachment, error) {
	_, _ = io.Copy(io.Discard, content)
	return nil, nil
}

func TestUploadClosesEveryHandleOnEveryPath(t *testing.T) {
	setup := func(t *testing.T) (string, string, string) {
		dir := t.TempDir()
		return writeFile(t, dir, "a.txt", "1"), writeFile(t, dir, "b.txt", "22"), dir
	}
	t.Run("success", func(t *testing.T) {
		opened := trackOpens(t)
		a, b, _ := setup(t)
		if _, err := Upload(enabled(100), "K-1", []string{a, b}); err != nil {
			t.Fatal(err)
		}
		if len(opened()) == 0 {
			t.Fatal("no handle observed")
		}
		assertAllClosed(t, opened())
	})
	t.Run("later path invalid", func(t *testing.T) {
		opened := trackOpens(t)
		a, _, dir := setup(t)
		if _, err := Upload(enabled(100), "K-1", []string{a, dir}); err == nil {
			t.Fatal("want error")
		}
		assertAllClosed(t, opened())
	})
	t.Run("settings error", func(t *testing.T) {
		opened := trackOpens(t)
		a, _, _ := setup(t)
		fake := &fakeUploader{settingsErr: errors.New("boom")}
		_, _ = Upload(fake, "K-1", []string{a})
		assertAllClosed(t, opened())
	})
	t.Run("attachments disabled", func(t *testing.T) {
		opened := trackOpens(t)
		a, _, _ := setup(t)
		_, _ = Upload(&fakeUploader{}, "K-1", []string{a})
		assertAllClosed(t, opened())
	})
	t.Run("over limit", func(t *testing.T) {
		opened := trackOpens(t)
		a, b, _ := setup(t)
		_, _ = Upload(enabled(1), "K-1", []string{a, b})
		assertAllClosed(t, opened())
	})
	t.Run("upload failure leaves later files closed", func(t *testing.T) {
		opened := trackOpens(t)
		a, b, _ := setup(t)
		fake := enabled(100)
		fake.failOn = "a.txt"
		_, _ = Upload(fake, "K-1", []string{a, b})
		assertAllClosed(t, opened())
	})
}

func TestUploadRejectsANamedPipeWithoutBlockingOnOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipe")
	if err := mkfifo(path); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Upload(enabled(100), "K-1", []string{path})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("err = %v, want a regular-file rejection", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Upload blocked opening a named pipe")
	}
}

func TestUploadRejectsAPathSwappedForANamedPipeBeforeOpen(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.txt", "regular")
	orig := openFile
	t.Cleanup(func() { openFile = orig })
	openFile = func(name string) (*os.File, error) {
		// The path is a regular file when inspected and a FIFO when opened.
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
		if err := mkfifo(name); err != nil {
			t.Skipf("mkfifo unavailable: %v", err)
		}
		return orig(name)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Upload(enabled(100), "K-1", []string{path})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("err = %v, want a regular-file rejection", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Upload blocked opening a path swapped for a named pipe")
	}
}

// leakyUploader hands the reader to a goroutine that outlives AddAttachment,
// like a client that returns without waiting for its body copier.
type leakyUploader struct {
	*fakeUploader
	gate chan struct{}
	done chan error
}

func (l *leakyUploader) AddAttachment(_, _ string, content io.Reader) ([]jira.Attachment, error) {
	go func() {
		<-l.gate
		_, err := content.Read(make([]byte, 8))
		l.done <- err
	}()
	return nil, nil
}

func TestUploadStopsServingTheStreamOnceTheClientReturns(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.txt", "abc")
	l := &leakyUploader{fakeUploader: enabled(100), gate: make(chan struct{}), done: make(chan error, 1)}

	if _, err := Upload(l, "K-1", []string{path}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	close(l.gate)
	if err := <-l.done; !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("late read error = %v, want io.ErrClosedPipe and no access to the closed handle", err)
	}
}

func TestUploadKeepsTheFileJiraAcceptedWhenTheClientReportsAnAppliedError(t *testing.T) {
	dir := t.TempDir()
	a, b := writeFile(t, dir, "a.txt", "1"), writeFile(t, dir, "b.txt", "2")
	up := &appliedUploader{fakeUploader: enabled(100)}

	results, err := Upload(up, "K-1", []string{a, b})
	if err == nil || jira.MutationStateOf(err) != jira.MutationApplied {
		t.Fatalf("err=%v state=%v, want an applied error", err, jira.MutationStateOf(err))
	}
	if len(results) != 2 || results[1].Path != b || len(results[1].Attachments) != 1 || results[1].Attachments[0].ID != "9" {
		t.Fatalf("results = %+v, want the confirmed applied file with its details", results)
	}
}

// appliedUploader accepts every file but fails the second with an applied error.
type appliedUploader struct {
	*fakeUploader
	calls int
}

func (u *appliedUploader) AddAttachment(_, filename string, content io.Reader) ([]jira.Attachment, error) {
	_, _ = io.Copy(io.Discard, content)
	u.calls++
	if u.calls == 2 {
		return []jira.Attachment{{ID: "9", Filename: filename}}, &jira.MutationError{State: jira.MutationApplied, Err: errors.New("response unreadable")}
	}
	return []jira.Attachment{{ID: "1", Filename: filename}}, nil
}

// mkfifo creates a named pipe through the platform tool so the tests compile
// everywhere; it errors where the tool does not exist.
func mkfifo(path string) error {
	return exec.Command("mkfifo", path).Run()
}

// errAfter yields its data, then fails with a non-EOF error.
type errAfter struct {
	data []byte
	err  error
}

func (e *errAfter) Read(p []byte) (int, error) {
	if len(e.data) > 0 {
		n := copy(p, e.data)
		e.data = e.data[n:]
		return n, nil
	}
	return 0, e.err
}

func TestBoundedReaderPropagatesANonEOFErrorFromTheGrowthProbe(t *testing.T) {
	boom := errors.New("disk failure")
	b := &boundedReader{r: &errAfter{data: []byte("abc"), err: boom}, remaining: 3, path: "a.txt"}

	got, err := io.ReadAll(b)
	if string(got) != "abc" || !errors.Is(err, boom) {
		t.Fatalf("got=%q err=%v, want the bytes and the original read error, not a clean EOF", got, err)
	}
	if !errors.Is(b.finish(), boom) {
		t.Fatalf("finish() = %v, want the probe error recorded", b.finish())
	}
}

// blockingReader allows the caller to finish while a Read is still in flight.
type blockingReader struct {
	entered chan struct{}
	release chan struct{}
}

func (r *blockingReader) Read(p []byte) (int, error) {
	close(r.entered)
	<-r.release
	return 0, io.EOF
}
func TestBoundedReaderFinishDoesNotWaitForBlockedRead(t *testing.T) {
	r := &blockingReader{entered: make(chan struct{}), release: make(chan struct{})}
	b := &boundedReader{r: r, remaining: 1, path: "file"}
	readDone := make(chan struct{})
	go func() { defer close(readDone); _, _ = b.Read(make([]byte, 1)) }()
	<-r.entered
	finished := make(chan struct{})
	go func() { _ = b.finish(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(250 * time.Millisecond):
		close(r.release)
		<-readDone
		<-finished
		t.Fatal("finish blocked on an in-flight read")
	}
	close(r.release)
	<-readDone
	if _, err := b.Read(make([]byte, 1)); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("read after finish: %v", err)
	}
}
