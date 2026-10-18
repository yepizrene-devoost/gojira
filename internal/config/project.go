package config

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
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

// ResolveProject uses the process environment and working directory. Git is read
// through its CLI so linked worktrees and submodules retain Git's scope rules.
func ResolveProject(flag, user string) (*ResolvedProject, error) {
	return ResolveProjectWith(flag, user, os.Getenv, func() ([]byte, error) {
		if err := exec.Command("git", "rev-parse", "--git-dir").Run(); err != nil {
			return nil, err
		}
		return exec.Command("git", "config", "--show-scope", "--get", "gojira.project").Output()
	})
}

// ResolveProjectWith injects environment lookup and Git execution for callers
// that need deterministic resolution. Empty layers fall through; invalid layers
// never silently fall back. An unavailable Git configuration is treated as unset.
func ResolveProjectWith(flag, user string, env func(string) string, git func() ([]byte, error)) (*ResolvedProject, error) {
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
	if output, err := git(); err == nil {
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
