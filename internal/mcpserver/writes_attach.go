package mcpserver

import (
	"io"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yepizrene-devoost/gojira/internal/attachments"
	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// uploadRecorder observes the shared upload flow. A failure before the first
// request is sent changed nothing in Jira, so it is reported as not applied;
// once a request is dispatched, its outcome is classified by MutationStateOf.
type uploadRecorder struct {
	attachments.Uploader
	settingsErr error
	dispatched  bool
}

func (r *uploadRecorder) GetAttachmentSettings() (jira.AttachmentSettings, error) {
	settings, err := r.Uploader.GetAttachmentSettings()
	r.settingsErr = err
	return settings, err
}

func (r *uploadRecorder) AddAttachment(issueKey, filename string, content io.Reader) ([]jira.Attachment, error) {
	r.dispatched = true
	return r.Uploader.AddAttachment(issueKey, filename, content)
}

type attachmentData struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// attachFile uploads one local file through the shared attachments flow.
func (a adapter) attachFile(key, path string) *mcp.CallToolResult {
	uploader, ok := a.Writer.(attachments.Uploader)
	if !ok {
		return writeResult("attach", key, jira.MutationNotApplied, nil, "configuration_error", "Attachment uploader unavailable", "Check server configuration.")
	}
	recorder := &uploadRecorder{Uploader: uploader}
	results, err := attachments.Upload(recorder, key, []string{path})
	if err == nil {
		var uploaded []attachmentData
		for _, result := range results {
			for _, attachment := range result.Attachments {
				uploaded = append(uploaded, attachmentData{ID: attachment.ID, Filename: attachment.Filename, Size: attachment.Size})
			}
		}
		return writeResult("attach", key, jira.MutationApplied, map[string]any{"attachments": uploaded}, "", "", "")
	}
	if recorder.dispatched {
		state := jira.MutationStateOf(err)
		if state == jira.MutationApplied {
			// Jira accepted the file; the failure came afterwards. Keep what is
			// known and never suggest uploading it again.
			var uploaded []attachmentData
			files := []string{}
			for _, result := range results {
				files = append(files, filepath.Base(result.Path))
				for _, attachment := range result.Attachments {
					uploaded = append(uploaded, attachmentData{ID: attachment.ID, Filename: attachment.Filename, Size: attachment.Size})
				}
			}
			data := map[string]any{"files": files, "attachments": uploaded}
			return writeResult("attach", key, state, data, "mutation_applied", "Jira accepted the attachment, but the upload did not finish cleanly",
				"The attachment is already in Jira; do not upload it again.", jira.MutationDiagnostics(err))
		}
		hint := "Check Jira access before retrying."
		if state == jira.MutationUnknown {
			hint = "Verify the attachment in Jira before retrying; outcome unknown. Never retry automatically."
		}
		return writeResult("attach", key, state, nil, "mutation_failed", "Jira operation failed", hint, jira.MutationDiagnostics(err))
	}
	if recorder.settingsErr != nil {
		return writeResult("attach", key, jira.MutationNotApplied, nil, "read_failed", "Attachment settings could not be read", "Check Jira access before retrying; nothing was uploaded.")
	}
	return writeResult("attach", key, jira.MutationNotApplied, nil, "validation_error", err.Error(), "Correct the path or the attachment settings before retrying; nothing was uploaded.")
}
