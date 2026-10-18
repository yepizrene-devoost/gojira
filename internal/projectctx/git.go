package projectctx

import "os/exec"

// CommandGitSource uses Git's scope rules for repositories, worktrees and submodules.
type CommandGitSource struct{}

func (CommandGitSource) ReadProject() ([]byte, error) {
	if err := exec.Command("git", "rev-parse", "--git-dir").Run(); err != nil {
		return nil, err
	}
	return exec.Command("git", "config", "--show-scope", "--get", "gojira.project").Output()
}
