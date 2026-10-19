package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yepizrene-devoost/gojira/internal/agentguide"
)

func TestGuideInstallPreview(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	target := filepath.Join(root, "new", "guide.md")
	command := newGuideCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetArgs([]string{"install", "--path", target})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Preview", target, fmt.Sprintf("%d bytes", len(agentguide.Content()))} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %q", want, out.String())
		}
	}
	if _, err := os.Stat(filepath.Dir(target)); !os.IsNotExist(err) {
		t.Fatalf("preview created directory: %v", err)
	}
}

func TestGuideInstallPreviewPrintsExclusion(t *testing.T) {
	if testing.Short() {
		t.Skip("temporary Git integration")
	}
	root := t.TempDir()
	t.Chdir(root)
	if data, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, data)
	}
	command := newGuideCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetArgs([]string{"install", "--path", "docs/guide.md", "--exclude"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Exclude: /docs/guide.md\n") {
		t.Fatalf("missing exclusion line: %q", out.String())
	}
}

func TestGuideInstallConfirmed(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	target := filepath.Join(root, "new", "guide.md")
	command := newGuideCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetArgs([]string{"install", "--path", target, "--yes"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != agentguide.Content() {
		t.Fatal("wrong guide content")
	}
	if !strings.Contains(out.String(), "Installed:") || strings.Contains(out.String(), "Preview:") {
		t.Fatalf("misleading confirmation: %q", out.String())
	}
}

func TestGuidePrintsOnlyContent(t *testing.T) {
	t.Chdir(t.TempDir())
	command := newGuideCommand()
	var out, diagnostics bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&diagnostics)
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != agentguide.Content() {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", diagnostics.String())
	}
}
