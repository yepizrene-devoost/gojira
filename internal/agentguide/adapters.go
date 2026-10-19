package agentguide

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runGit runs git with a stable locale. Only Git's own "not a git repository"
// answer maps to ErrOutsideWorkingTree; any other failure, including a missing
// git executable, is reported as an undetermined repository state.
func runGit(directory string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	out, err := command.Output()
	if err == nil {
		return out, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 128 && strings.Contains(stderr.String(), "not a git repository") {
		return nil, ErrOutsideWorkingTree
	}
	return nil, errors.New("cannot run git")
}

// GitAdapter resolves Git metadata through Git itself, including in worktrees.
type GitAdapter struct{}

func (GitAdapter) Prepare(target string) (Exclusion, error) {
	absolute, err := filepath.Abs(target)
	if err != nil {
		return Exclusion{}, errors.New("cannot resolve guide path")
	}
	directory := filepath.Dir(absolute)
	for {
		info, statErr := os.Stat(directory)
		if statErr == nil && info.IsDir() {
			break
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return Exclusion{}, ErrOutsideWorkingTree
		}
		directory = parent
	}
	suffix, err := filepath.Rel(directory, absolute)
	if err != nil {
		return Exclusion{}, errors.New("cannot resolve guide path")
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		return Exclusion{}, errors.New("cannot resolve guide directory")
	}
	absolute = filepath.Join(directory, suffix)
	rootBytes, err := runGit(directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return Exclusion{}, err
	}
	root := strings.TrimSpace(string(rootBytes))
	relative, err := filepath.Rel(root, absolute)
	if err != nil {
		return Exclusion{}, errors.New("cannot resolve relative guide path")
	}
	excludeBytes, err := exec.Command("git", "-C", root, "rev-parse", "--git-path", "info/exclude").Output()
	if err != nil {
		return Exclusion{}, errors.New("cannot resolve Git exclude file")
	}
	exclude := strings.TrimSpace(string(excludeBytes))
	if !filepath.IsAbs(exclude) {
		exclude = filepath.Join(root, exclude)
	}
	line, err := ExcludePattern(filepath.ToSlash(relative))
	if err != nil {
		return Exclusion{}, err
	}
	return Exclusion{File: exclude, Line: line, Root: root, Relative: filepath.ToSlash(relative)}, nil
}

func (GitAdapter) Apply(exclusion Exclusion) error {
	data, err := os.ReadFile(exclusion.File)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errors.New("cannot read Git exclude file")
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == exclusion.Line {
			return nil
		}
	}
	addition := exclusion.Line + "\n"
	if len(data) > 0 && data[len(data)-1] != '\n' {
		addition = "\n" + addition
	}
	file, err := os.OpenFile(exclusion.File, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return errors.New("cannot open Git exclude file")
	}
	if _, err := file.WriteString(addition); err != nil {
		_ = file.Close()
		return errors.New("cannot update Git exclude file")
	}
	if err := file.Close(); err != nil {
		return errors.New("cannot close Git exclude file")
	}
	return nil
}

// Ignored asks Git whether the target is effectively ignored (exit 0 = ignored,
// exit 1 = not ignored); any other outcome leaves the state undetermined.
func (GitAdapter) Ignored(exclusion Exclusion) (bool, error) {
	command := exec.Command("git", "-C", exclusion.Root, "check-ignore", "-q", "--", exclusion.Relative)
	command.Env = append(os.Environ(), "LC_ALL=C")
	err := command.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, errors.New("cannot verify the effective Git ignore status")
}

// OSFilesystem provides the real destination filesystem.
type OSFilesystem struct{}

func (OSFilesystem) Lstat(path string) (fs.FileInfo, error) { return os.Lstat(path) }

func (OSFilesystem) MkdirAll(path string, mode fs.FileMode) error { return os.MkdirAll(path, mode) }

func (OSFilesystem) WriteExclusive(path string, data []byte, mode fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
