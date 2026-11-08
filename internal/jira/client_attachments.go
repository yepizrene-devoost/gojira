package jira

import (
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
)

// AttachmentSettings is the instance-wide attachment configuration.
type AttachmentSettings struct {
	Enabled     bool  `json:"enabled"`
	UploadLimit int64 `json:"uploadLimit"`
}

// Attachment is the part of an uploaded attachment that GoJira reports.
type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
}

// GetAttachmentSettings reads whether attachments are enabled and the largest
// size Jira accepts, so an upload that cannot succeed is refused up front.
func (c *Client) GetAttachmentSettings() (AttachmentSettings, error) {
	b, err := c.get(pathAttachmentMeta)
	if err != nil {
		return AttachmentSettings{}, err
	}
	var settings AttachmentSettings
	if err := json.Unmarshal(b, &settings); err != nil {
		return AttachmentSettings{}, err
	}
	return settings, nil
}

// AddAttachment uploads one file to an issue as a multipart request. The file
// is streamed, never buffered, and the request is dispatched exactly once: a
// repeated upload would attach the file twice.
func (c *Client) AddAttachment(issueKey, filename string, content io.Reader) ([]Attachment, error) {
	pr, pw := io.Pipe()
	form := multipart.NewWriter(pw)
	go func() {
		part, err := form.CreateFormFile("file", filename)
		if err == nil {
			_, err = io.Copy(part, content)
		}
		if err == nil {
			err = form.Close()
		}
		_ = pw.CloseWithError(err)
	}()

	req, err := http.NewRequest(http.MethodPost, c.baseURL+issuePath(pathIssueAttachments, issueKey), pr)
	if err != nil {
		_ = pr.Close()
		return nil, &MutationError{State: MutationNotApplied, Err: errors.New("invalid Jira mutation request")}
	}
	req.SetBasicAuth(c.auth, "")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("X-Atlassian-Token", "no-check")

	body, err := c.dispatchMutation(req)
	_ = pr.Close()
	if err != nil {
		return nil, err
	}
	// Jira already accepted the file. A body that cannot be read only loses the
	// attachment details, so it must not turn an applied upload into a failure.
	var uploaded []Attachment
	if err := json.Unmarshal(body, &uploaded); err != nil {
		return nil, nil
	}
	return uploaded, nil
}
