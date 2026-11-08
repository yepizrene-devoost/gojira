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

type file struct {
	path string
	name string
	size int64
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
			return results, fmt.Errorf("uploading %s: %w", f.path, err)
		}
		results = append(results, Result{Path: f.path, Attachments: uploaded})
	}
	return results, nil
}

func send(client Uploader, issueKey string, f file) ([]jira.Attachment, error) {
	handle, err := os.Open(f.path)
	if err != nil {
		return nil, &jira.MutationError{State: jira.MutationNotApplied, Err: err}
	}
	defer func() { _ = handle.Close() }()
	return client.AddAttachment(issueKey, f.name, handle)
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
	info, err := os.Stat(path)
	if err != nil {
		return file{}, fmt.Errorf("%s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return file{}, fmt.Errorf("%s is not a regular file", path)
	}
	handle, err := os.Open(path)
	if err != nil {
		return file{}, fmt.Errorf("%s: %w", path, err)
	}
	_ = handle.Close()
	return file{path: path, name: filepath.Base(path), size: info.Size()}, nil
}
