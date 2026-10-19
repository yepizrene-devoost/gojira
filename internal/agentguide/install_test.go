package agentguide

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPreviewExclusionWritesNothing(t *testing.T) {
	root := repository(t)
	target := filepath.Join(root, "new", "guide.md")
	exclusion, err := (GitAdapter{}).Prepare(target)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(exclusion.File)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Install(OSFilesystem{}, GitAdapter{}, Options{Path: target, Exclude: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Exclusion != "/new/guide.md" {
		t.Fatalf("missing preview exclusion: %+v", plan)
	}
	after, err := os.ReadFile(exclusion.File)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("preview changed exclusions")
	}
	if _, err := os.Stat(filepath.Dir(target)); !os.IsNotExist(err) {
		t.Fatalf("preview created directory: %v", err)
	}
}

func TestInstallConfirmedExcludesBeforeWriting(t *testing.T) {
	root := repository(t)
	target := filepath.Join(root, "guide.md")
	if _, err := Install(OSFilesystem{}, GitAdapter{}, Options{Path: target, Exclude: true, Yes: true}); err != nil {
		t.Fatal(err)
	}
	exclusion, err := (GitAdapter{}).Prepare(target)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exclusion.File)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\n/guide.md\n") {
		t.Fatalf("exclusion missing: %q", data)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal(err)
	}
}

type failedExclusion struct{}

func (failedExclusion) Prepare(string) (Exclusion, error) { return Exclusion{Line: "guide.md"}, nil }
func (failedExclusion) Ignored(Exclusion) (bool, error)   { return true, nil }
func (failedExclusion) Apply(Exclusion) error {
	return errors.New("private absolute path must not be exposed")
}

func TestInstallExclusionFailureLeavesNoGuide(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "new", "guide.md")
	_, err := Install(OSFilesystem{}, failedExclusion{}, Options{Path: target, Yes: true, Exclude: true})
	if err == nil || !strings.Contains(err.Error(), "exclusion") || strings.Contains(err.Error(), "private") {
		t.Fatalf("unsafe or missing error: %v", err)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("half-done installation: %v %v", entries, readErr)
	}
}

func TestInstallOutsideRepositoryWritesNothing(t *testing.T) {
	if testing.Short() {
		t.Skip("Git integration")
	}
	root := t.TempDir()
	for _, yes := range []bool{false, true} {
		_, err := Install(OSFilesystem{}, GitAdapter{}, Options{Path: filepath.Join(root, "new", "guide.md"), Exclude: true, Yes: yes})
		if err == nil || !strings.Contains(err.Error(), "Git working tree") {
			t.Fatalf("missing Git error: %v", err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside installation wrote files: %v %v", entries, err)
	}
}

func TestInstallRefusalsPreserveExistingTargets(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "guide.md")
	if err := os.WriteFile(existing, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, yes := range []bool{false, true} {
		for _, path := range []string{existing, root, "", filepath.Join(root, "aGeNtS.md"), filepath.Join(root, "cLaUdE.md")} {
			if _, err := Install(OSFilesystem{}, nil, Options{Path: path, Yes: yes}); err == nil {
				t.Fatalf("accepted %q", path)
			}
		}
	}
	data, err := os.ReadFile(existing)
	if err != nil || string(data) != "original" {
		t.Fatalf("existing target changed: %q %v", data, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("refusal created files: %v %v", entries, err)
	}
}

func TestInstallConfirmedWritesGuide(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "new", "guide.md")
	if _, err := Install(OSFilesystem{}, nil, Options{Path: target, Yes: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != Content() {
		t.Fatal("incorrect content")
	}
	for path, want := range map[string]os.FileMode{target: 0644, filepath.Dir(target): 0755} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Fatalf("mode: %v; want %v", info.Mode().Perm(), want)
		}
	}
}

func TestInstallPreviewWritesNothing(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "new", "guide.md")
	plan, err := Install(OSFilesystem{}, nil, Options{Path: target})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Path != target || plan.Size != len(Content()) {
		t.Fatalf("wrong preview: %+v", plan)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preview mutated filesystem: %v %v", entries, err)
	}
}
