package attachments

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

type fakeUploader struct {
	settings    jira.AttachmentSettings
	settingsErr error
	failOn      string
	uploads     []string
	contents    []string
	issue       string
}

func (f *fakeUploader) GetAttachmentSettings() (jira.AttachmentSettings, error) {
	return f.settings, f.settingsErr
}

func (f *fakeUploader) AddAttachment(issueKey, filename string, content io.Reader) ([]jira.Attachment, error) {
	f.issue = issueKey
	if filename == f.failOn {
		return nil, &jira.MutationError{State: jira.MutationUnknown, Err: errors.New("transport")}
	}
	body, _ := io.ReadAll(content)
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
