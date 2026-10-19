package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/config"
	"github.com/yepizrene-devoost/gojira/internal/jira"
	"github.com/yepizrene-devoost/gojira/internal/picker"
)

func TestProjectInitConfigCompatibility(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOJIRA_PROJECT", "")
	if err := configSetCmd.RunE(configSetCmd, []string{"project", "ABC"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil || cfg.Project != "ABC" {
		t.Fatalf("config=%+v err=%v", cfg, err)
	}
	command, _, err := rootCmd.Find([]string{"config", "init"})
	if err != nil || command != configInitCmd || command == newProjectInitCommand() {
		t.Fatalf("config init registration changed: %v", err)
	}
}

func TestProjectInit(t *testing.T) {
	for _, mode := range []string{"outside", "missing-yes", "missing-project", "success", "tty-key", "picker", "cancel", "empty", "unknown", "forbidden", "invalid", "override"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "global"))
			t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(home, "system"))
			t.Setenv("GOJIRA_PROJECT", "")
			t.Chdir(t.TempDir())
			git := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", args...).CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %s: %v", args, out, err)
				}
				return string(out)
			}
			if mode != "outside" {
				git("init", "--quiet")
			}
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if mode == "unknown" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if mode == "forbidden" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				if r.URL.Path == "/rest/api/3/project/search" {
					_, _ = fmt.Fprint(w, `{"values":[{"key":"ABC","name":"Alpha"}],"isLast":true}`)
					return
				}
				if r.URL.Path != "/rest/api/3/project/ABC" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				_, _ = fmt.Fprint(w, `{"key":"ABC","name":"Alpha"}`)
			}))
			defer server.Close()
			oldClient, oldTerminal, oldPicker := buildClient, isTerminal, pickProject
			t.Cleanup(func() { buildClient, isTerminal, pickProject = oldClient, oldTerminal, oldPicker })
			buildClient = func() (*jira.Client, string, error) { return jira.NewClient(server.URL, "email", "token"), "", nil }
			tty := mode == "tty-key" || mode == "picker" || mode == "cancel" || mode == "empty"
			isTerminal = func() bool { return tty }
			picked := false
			pickProject = func(items []picker.Item) (string, error) {
				picked = true
				if len(items) != 1 || items[0].Key != "ABC" {
					t.Fatalf("items=%v", items)
				}
				if mode == "cancel" {
					return "", picker.ErrCancelled
				}
				if mode == "empty" {
					return "", nil
				}
				return "ABC", nil
			}
			args := []string{"--json", "--project", "ABC", "--yes"}
			switch mode {
			case "missing-yes":
				args = []string{"--json", "--project", "ABC"}
			case "missing-project":
				args = []string{"--json", "--yes"}
			case "picker", "cancel", "empty":
				args = []string{"--json"}
			case "tty-key":
				args = []string{"--json", "--project", "ABC"}
			case "invalid":
				args = []string{"--json", "--project", "bad", "--yes"}
			case "override":
				t.Setenv("GOJIRA_PROJECT", "OTHER")
			}
			run := func() (string, string, error) {
				command := newProjectInitCommand()
				var out, stderr bytes.Buffer
				command.SetOut(&out)
				command.SetErr(&stderr)
				command.SilenceErrors = true
				command.SilenceUsage = true
				command.SetArgs(args)
				err := command.Execute()
				return out.String(), stderr.String(), err
			}
			out, stderr, err := run()
			success := mode == "success" || mode == "tty-key" || mode == "picker" || mode == "override"
			if !success {
				if err == nil {
					t.Fatal("expected refusal")
				}
				if mode != "outside" {
					if value, e := exec.Command("git", "config", "--local", "--get", "gojira.project").Output(); e == nil || len(value) != 0 {
						t.Fatalf("unexpected write %s", value)
					}
				}
				if mode == "outside" || mode == "missing-yes" || mode == "missing-project" || mode == "invalid" {
					if requests != 0 {
						t.Fatalf("requests=%d", requests)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("execute: %v stderr=%s", err, stderr)
			}
			if got := git("config", "--local", "--get", "gojira.project"); got != "ABC\n" {
				t.Fatalf("stored=%q", got)
			}
			var result struct {
				SchemaVersion string `json:"schemaVersion"`
				Project       string `json:"project"`
				Written       string `json:"written"`
				Effective     struct{ Key, Source, Scope string }
				Warning       string
			}
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			if result.SchemaVersion != "v1" || result.Project != "ABC" || result.Written != "local" {
				t.Fatalf("result=%+v", result)
			}
			if mode == "override" {
				if result.Effective.Key != "OTHER" || result.Effective.Source != "env" || result.Warning == "" {
					t.Fatalf("missing override: %+v", result)
				}
			} else if result.Effective.Key != "ABC" || result.Effective.Source != "git" || result.Effective.Scope != "local" {
				t.Fatalf("effective=%+v", result.Effective)
			}
			if picked != (mode == "picker") {
				t.Fatalf("picked=%t", picked)
			}
			if mode == "success" {
				before := git("config", "--local", "--list")
				if _, _, err := run(); err != nil {
					t.Fatal(err)
				}
				if after := git("config", "--local", "--list"); after != before {
					t.Fatal("not idempotent")
				}
			}
		})
	}
}
