// Package attachments holds the upload logic shared by the CLI, MCP and TUI so
// the three surfaces validate and report attachments identically.
package attachments

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// Uploader is the part of the Jira client the upload flow needs.
type Uploader interface {
	GetAttachmentSettings() (jira.AttachmentSettings, error)
	AddAttachment(issueKey, filename string, content io.Reader) ([]jira.Attachment, error)
}

// Result reports the outcome of one file.
type Result struct {
	Path        string
	Attachments []jira.Attachment
}

// openFile is the single place files are opened so tests can observe handles.
// It opens without blocking so a path swapped for a named pipe after it was
// checked cannot hang the open; the handle is then verified with Stat.
var openFile = func(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}

// file is a validated, open regular file. The handle that was checked is the
// handle that gets uploaded, so replacing the pathname afterwards cannot
// change what is sent.
type file struct {
	path   string
	name   string
	size   int64
	handle *os.File
}

func closeAll(files []file) {
	for _, f := range files {
		_ = f.handle.Close()
	}
}

// Upload validates every path and the instance limits, then uploads the files
// one request each. Nothing is uploaded when validation fails. After the first
// failed upload the remaining files are not attempted; the results of the files
// already uploaded are returned together with that error.
func Upload(client Uploader, issueKey string, paths []string) ([]Result, error) {
	if len(paths) == 0 {
		return nil, errors.New("at least one file path is required")
	}
	files := make([]file, 0, len(paths))
	defer func() { closeAll(files) }()
	for _, raw := range paths {
		f, err := inspect(raw)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	settings, err := client.GetAttachmentSettings()
	if err != nil {
		return nil, fmt.Errorf("reading attachment settings: %w", err)
	}
	if !settings.Enabled {
		return nil, errors.New("attachments are disabled on this Jira instance")
	}
	for _, f := range files {
		if f.size > settings.UploadLimit {
			return nil, fmt.Errorf("%s is %d bytes, over the Jira upload limit of %d bytes", f.path, f.size, settings.UploadLimit)
		}
	}

	results := make([]Result, 0, len(files))
	for _, f := range files {
		uploaded, err := send(client, issueKey, f)
		if err != nil {
			// Jira confirmed the file even though the call failed afterwards, so
			// the partial report must still name it.
			if jira.MutationStateOf(err) == jira.MutationApplied {
				results = append(results, Result{Path: f.path, Attachments: uploaded})
			}
			return results, fmt.Errorf("uploading %s: %w", f.path, err)
		}
		results = append(results, Result{Path: f.path, Attachments: uploaded})
	}
	return results, nil
}

// send uploads exactly the validated size from the validated handle. A file
// that grows or shrinks while streaming fails the upload instead of being
// silently truncated. Contents rewritten in place at the same size cannot be
// detected: the handle pins the file, not its bytes.
func send(client Uploader, issueKey string, f file) ([]jira.Attachment, error) {
	if _, err := f.handle.Seek(0, io.SeekStart); err != nil {
		return nil, &jira.MutationError{State: jira.MutationNotApplied, Err: err}
	}
	body := &boundedReader{r: f.handle, remaining: f.size, path: f.path}
	uploaded, err := client.AddAttachment(issueKey, f.name, body)
	// A client may leave a goroutine reading the stream; from here on it gets an
	// error instead of touching the handle that Upload is about to close.
	changed := body.finish()
	if changed != nil {
		// Bytes were already handed to the client, so the outcome depends on
		// what Jira did with a stream that ended in error.
		if err == nil {
			return uploaded, &jira.MutationError{State: jira.MutationApplied, Err: changed}
		}
		if state := jira.MutationStateOf(err); state != jira.MutationUnknown {
			return uploaded, err
		}
		return uploaded, &jira.MutationError{State: jira.MutationUnknown, Err: changed}
	}
	return uploaded, err
}

// boundedReader yields at most the validated number of bytes and reports a
// size change as an error rather than ending the stream early or late.
type boundedReader struct {
	mu        sync.Mutex
	readMu    sync.Mutex
	r         io.Reader
	remaining int64
	path      string
	changed   error
	finished  bool
}

// finish stops the stream and returns the size change observed, if any. It
// does not wait for a blocked Read; closing the underlying os.File is safe
// concurrently with its methods. An arbitrary reader may remain blocked.
func (b *boundedReader) finish() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.finished = true
	return b.changed
}

func (b *boundedReader) Read(p []byte) (int, error) {
	b.readMu.Lock()
	defer b.readMu.Unlock()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.finished {
		return 0, io.ErrClosedPipe
	}
	if b.changed != nil {
		return 0, b.changed
	}
	if b.remaining == 0 {
		var probe [1]byte
		b.mu.Unlock()
		n, err := b.r.Read(probe[:])
		b.mu.Lock()
		if b.finished {
			return 0, io.ErrClosedPipe
		}
		if n > 0 {
			b.changed = fmt.Errorf("%s grew after validation; refusing to upload a truncated file", b.path)
			return 0, b.changed
		}
		if err != nil && err != io.EOF {
			b.changed = fmt.Errorf("%s: checking for growth after validation: %w", b.path, err)
			return 0, b.changed
		}
		return 0, io.EOF
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	b.mu.Unlock()
	n, err := b.r.Read(p)
	b.mu.Lock()
	if b.finished {
		return 0, io.ErrClosedPipe
	}
	b.remaining -= int64(n)
	if err == io.EOF && b.remaining > 0 {
		b.changed = fmt.Errorf("%s shrank after validation; refusing to upload a truncated file", b.path)
		return n, b.changed
	}
	if err == io.EOF {
		err = nil
	} else if err != nil {
		b.changed = fmt.Errorf("%s: reading validated file: %w", b.path, err)
		return n, b.changed
	}
	return n, err
}

// inspect resolves a leading "~/" and requires a regular, readable file.
func inspect(raw string) (file, error) {
	path := raw
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return file{}, fmt.Errorf("%s: resolving home directory: %w", raw, err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	handle, err := openFile(path)
	if err != nil {
		return file{}, fmt.Errorf("%s: %w", path, err)
	}
	info, err := handle.Stat()
	if err != nil {
		_ = handle.Close()
		return file{}, fmt.Errorf("%s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		_ = handle.Close()
		return file{}, fmt.Errorf("%s is not a regular file", path)
	}
	return file{path: path, name: filepath.Base(path), size: info.Size(), handle: handle}, nil
}
