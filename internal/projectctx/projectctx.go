package projectctx

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/yepizrene-devoost/gojira/internal/jira"
)

// ProjectSource identifies the winning project configuration layer.
type ProjectSource string

const (
	ProjectSourceFlag ProjectSource = "flag"
	ProjectSourceEnv  ProjectSource = "env"
	ProjectSourceGit  ProjectSource = "git"
	ProjectSourceUser ProjectSource = "user"
)

// ResolvedProject describes the effective project without exposing credentials.
type ResolvedProject struct {
	Key    string        `json:"key"`
	Source ProjectSource `json:"source"`
	Scope  string        `json:"scope,omitempty"`
}

// ProjectValidationError names the invalid configuration layer, not its value.
type ProjectValidationError struct{ Source ProjectSource }

func (e *ProjectValidationError) Error() string {
	return fmt.Sprintf("invalid project key from %s: expected ^[A-Z][A-Z0-9_]*$", e.Source)
}

var projectKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// GitSource reads scoped project configuration. Unavailable configuration is unset.
type GitSource interface{ ReadProject() ([]byte, error) }

// ResolveProject uses the process environment and working directory.
func ResolveProject(flag, user string) (*ResolvedProject, error) {
	return ResolveProjectWith(flag, user, os.Getenv, CommandGitSource{})
}

// ResolveProjectWith injects environment and Git reads. Invalid layers never fall back.
func ResolveProjectWith(flag, user string, env func(string) string, git GitSource) (*ResolvedProject, error) {
	validate := func(key string, source ProjectSource, scope string) (*ResolvedProject, error) {
		if !projectKeyPattern.MatchString(key) {
			return nil, &ProjectValidationError{Source: source}
		}
		return &ResolvedProject{Key: key, Source: source, Scope: scope}, nil
	}
	if flag != "" {
		return validate(flag, ProjectSourceFlag, "")
	}
	if value := env("GOJIRA_PROJECT"); value != "" {
		return validate(value, ProjectSourceEnv, "")
	}
	if output, err := git.ReadProject(); err == nil {
		scope, value, ok := strings.Cut(strings.TrimSuffix(string(output), "\n"), "\t")
		if ok && value != "" {
			return validate(value, ProjectSourceGit, scope)
		}
	}
	if user != "" {
		return validate(user, ProjectSourceUser, "")
	}
	return nil, nil
}

// IdentityReader supplies the authenticated identity.
type IdentityReader interface{ GetMyself() (*jira.UserRef, error) }

// ContextIdentity deliberately omits private fields from Jira's UserRef.
type ContextIdentity struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}

type ContextJSON struct {
	SchemaVersion string           `json:"schemaVersion"`
	Project       *ResolvedProject `json:"project"`
	NeedsProject  bool             `json:"needsProject"`
	Identity      ContextIdentity  `json:"identity"`
}

// ReadContext resolves a project before reading the authenticated identity.
func ReadContext(client IdentityReader, flag, userDefault, schemaVersion string) (*ContextJSON, error) {
	project, err := ResolveProject(flag, userDefault)
	if err != nil {
		return nil, err
	}
	user, err := client.GetMyself()
	if err != nil {
		return nil, err
	}
	return &ContextJSON{SchemaVersion: schemaVersion, Project: project, NeedsProject: project == nil, Identity: ContextIdentity{AccountID: user.AccountID, DisplayName: user.DisplayName, Active: user.Active}}, nil
}
