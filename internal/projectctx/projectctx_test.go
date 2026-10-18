package projectctx

import (
	"encoding/json"
	"errors"
	"github.com/yepizrene-devoost/gojira/internal/jira"
	"os/exec"
	"strings"
	"testing"
)

type identityStub struct {
	calls int
	err   error
}

func (s *identityStub) GetMyself() (*jira.UserRef, error) {
	s.calls++
	return &jira.UserRef{AccountID: "id", DisplayName: "Person", Active: true}, s.err
}

func TestReadContext(t *testing.T) {
	t.Setenv("GOJIRA_PROJECT", "")
	t.Setenv("PATH", "")
	for _, key := range []string{"ABC", ""} {
		identity := &identityStub{}
		result, err := ReadContext(identity, key, "", "v1")
		if err != nil || identity.calls != 1 {
			t.Fatalf("result=%+v err=%v calls=%d", result, err, identity.calls)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		project := `null`
		if key != "" {
			project = `{"key":"ABC","source":"flag"}`
		}
		want := `{"schemaVersion":"v1","project":` + project + `,"needsProject":`
		if key == "" {
			want += "true"
		} else {
			want += "false"
		}
		want += `,"identity":{"accountId":"id","displayName":"Person","active":true}}`
		if string(data) != want || strings.Contains(string(data), "email") {
			t.Fatalf("context=%s", data)
		}
	}
	identity := &identityStub{}
	if _, err := ReadContext(identity, "bad", "", "v1"); err == nil || identity.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, identity.calls)
	}
	failure := errors.New("identity unavailable")
	identity.err = failure
	if _, err := ReadContext(identity, "ABC", "", "v1"); !errors.Is(err, failure) {
		t.Fatalf("err=%v", err)
	}
}

type gitFunc func() ([]byte, error)

func (f gitFunc) ReadProject() ([]byte, error) { return f() }

func TestResolveProject(t *testing.T) {
	for _, tc := range []struct {
		name, flag, env, git, user, key, source, scope string
		invalid                                        bool
	}{
		{name: "flag", flag: "FLAG", env: "ENV", git: "local\tGIT", user: "USER", key: "FLAG", source: "flag"},
		{name: "env", env: "ENV", git: "local\tGIT", user: "USER", key: "ENV", source: "env"},
		{name: "git", git: "worktree\tGIT\n", user: "USER", key: "GIT", source: "git", scope: "worktree"},
		{name: "user", user: "USER", key: "USER", source: "user"},
		{name: "missing"},
		{name: "empty git", git: "local\t\n", user: "USER", key: "USER", source: "user"},
		{name: "invalid flag", flag: "bad", env: "ENV", invalid: true, source: "flag"},
		{name: "invalid env", env: " bad ", user: "USER", invalid: true, source: "env"},
		{name: "invalid git", git: "local\tbad", user: "USER", invalid: true, source: "git"},
		{name: "invalid user", user: "bad", invalid: true, source: "user"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ResolveProjectWith(tc.flag, tc.user, func(string) string { return tc.env }, gitFunc(func() ([]byte, error) { return []byte(tc.git), nil }))
			if tc.invalid {
				var invalid *ProjectValidationError
				if !errors.As(err, &invalid) || string(invalid.Source) != tc.source {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.key == "" {
				if p != nil {
					t.Fatalf("project=%+v", p)
				}
				return
			}
			if p == nil || p.Key != tc.key || string(p.Source) != tc.source || p.Scope != tc.scope {
				t.Fatalf("project=%+v", p)
			}
		})
	}
}

func TestResolveProjectGitUnavailable(t *testing.T) {
	for _, err := range []error{exec.ErrNotFound, errors.New("exit status 1"), errors.New("not a repository")} {
		p, e := ResolveProjectWith("", "USER", func(string) string { return "" }, gitFunc(func() ([]byte, error) { return nil, err }))
		if e != nil || p == nil || p.Key != "USER" {
			t.Fatalf("project=%+v error=%v", p, e)
		}
	}
}
