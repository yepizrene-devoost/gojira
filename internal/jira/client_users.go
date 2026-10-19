package jira

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

func (c *Client) GetMyself() (*UserRef, error) {
	b, err := c.get(pathMyself)
	if err != nil {
		return nil, err
	}
	var user UserRef
	if err := json.Unmarshal(b, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

type lookupError struct {
	message string
	cause   error
}

func (e *lookupError) Error() string { return e.message }
func (e *lookupError) Unwrap() error { return e.cause }

// ResolveAccountID looks up a Jira user by email and returns their
// accountId and display name.
//
// Atlassian removed /rest/api/3/user/username and stopped exposing emails in
// users/search, so resolution goes:
//  1. v2 user picker — matches the email query server-side;
//  2. exact verification via GET /user?accountId (emailAddress compare);
//  3. JQL fallback — an issue where the user is creator/reporter/assignee
//     also resolves the email server-side.
func (c *Client) ResolveAccountID(email string) (accountID, displayName string, err error) {
	cands, pickerErr := c.userPicker(email)
	if pickerErr == nil {
		for _, u := range cands {
			if u.AccountID == "" {
				continue
			}
			full, err := c.getUser(u.AccountID)
			if err != nil {
				// Cannot verify: accept a sole picker match rather than fail.
				if len(cands) == 1 {
					return u.AccountID, u.DisplayName, nil
				}
				continue
			}
			if full.EmailAddress == "" && len(cands) == 1 {
				// Site hides emails from the API; trust the sole match.
				return full.AccountID, full.DisplayName, nil
			}
			if strings.EqualFold(full.EmailAddress, email) {
				return full.AccountID, full.DisplayName, nil
			}
		}
	}

	// The JQL fallback is best-effort: matching nobody is a normal miss, but a
	// transport or decode failure must not be reported as "no such user".
	jqlUser, jqlErr := c.resolveViaJQL(email)
	if jqlUser != nil {
		return jqlUser.AccountID, jqlUser.DisplayName, nil
	}

	var reasons []string
	if pickerErr != nil {
		reasons = append(reasons, fmt.Sprintf("picker: %v", pickerErr))
	}
	if jqlErr != nil {
		reasons = append(reasons, fmt.Sprintf("jql: %v", jqlErr))
	}
	if len(reasons) > 0 {
		message := fmt.Sprintf("no user found for %s (%s)", email, strings.Join(reasons, "; "))
		return "", "", &lookupError{message: message, cause: errors.Join(pickerErr, jqlErr)}
	}
	return "", "", fmt.Errorf("no user found for %s", email)
}

// userPicker returns account candidates for a free-form query (email or name).
func (c *Client) userPicker(query string) ([]UserRef, error) {
	b, err := c.get(fmt.Sprintf(pathUserPicker, url.QueryEscape(query)))
	if err != nil {
		return nil, err
	}
	var r struct {
		Users []UserRef `json:"users"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return r.Users, nil
}

// getUser reads a user by accountId, including emailAddress when visible.
func (c *Client) getUser(accountID string) (*UserRef, error) {
	b, err := c.get(fmt.Sprintf(pathUserByAccountID, url.QueryEscape(accountID)))
	if err != nil {
		return nil, err
	}
	var u UserRef
	if err := json.Unmarshal(b, &u); err != nil {
		return nil, err
	}
	if u.AccountID == "" {
		return nil, fmt.Errorf("user %s not found", accountID)
	}
	return &u, nil
}

// resolveViaJQL finds a user through any issue they touch. Jira resolves the
// email server-side; the returned user objects carry the email for an exact
// verification. A nil user with a nil error means the fallback ran and matched
// nobody; a non-nil error means it could not be evaluated at all.
func (c *Client) resolveViaJQL(email string) (*UserRef, error) {
	if email == "" || strings.ContainsAny(email, "\"\\") {
		return nil, nil // not a real email; never break the JQL string
	}
	jql := fmt.Sprintf(`creator = "%[1]s" OR reporter = "%[1]s" OR assignee = "%[1]s"`, email)
	path := fmt.Sprintf("%s?jql=%s&maxResults=1&fields=creator,reporter,assignee",
		pathSearchJQL, url.QueryEscape(jql))
	b, err := c.get(path)
	if err != nil {
		return nil, err
	}
	var r struct {
		Issues []struct {
			Fields struct {
				Creator  *UserRef `json:"creator"`
				Reporter *UserRef `json:"reporter"`
				Assignee *UserRef `json:"assignee"`
			} `json:"fields"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	for _, iss := range r.Issues {
		for _, u := range []*UserRef{iss.Fields.Creator, iss.Fields.Reporter, iss.Fields.Assignee} {
			if u != nil && u.AccountID != "" && strings.EqualFold(u.EmailAddress, email) {
				return u, nil
			}
		}
	}
	return nil, nil
}
