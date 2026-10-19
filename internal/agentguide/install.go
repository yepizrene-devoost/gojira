package agentguide

import (
	"errors"
	"io/fs"
	"path/filepath"
)

// Options expresses the user's explicit installation choices.
type Options struct {
	Path    string
	Yes     bool
	Exclude bool
}

// Install validates and previews an optional guide installation.
func Install(files Filesystem, git GitExclude, options Options) (InstallPlan, error) {
	info, err := files.Lstat(options.Path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return InstallPlan{}, errors.New("cannot inspect guide target")
	}
	plan, err := Plan(options.Path, err == nil, info != nil && info.IsDir())
	if err != nil {
		return plan, err
	}
	var exclusion Exclusion
	if options.Exclude {
		if git == nil {
			return InstallPlan{}, errors.New("git exclusion adapter is required")
		}
		exclusion, err = git.Prepare(plan.Path)
		if err != nil {
			return InstallPlan{}, errors.New("cannot prepare Git exclusion; target must be inside a Git working tree")
		}
		plan.Exclusion = exclusion.Line
	}
	if !options.Exclude && git != nil {
		_, prepareErr := git.Prepare(plan.Path)
		if prepareErr == nil {
			return InstallPlan{}, errors.New("target is inside a Git working tree; pass --exclude to keep the guide untracked, or choose a path outside the repository")
		}
		if !errors.Is(prepareErr, ErrOutsideWorkingTree) {
			return InstallPlan{}, errors.New("cannot determine whether the target is inside a Git working tree; refusing to install")
		}
	}
	if !options.Yes {
		return plan, nil
	}
	if options.Exclude {
		if err := git.Apply(exclusion); err != nil {
			return InstallPlan{}, errors.New("cannot apply Git exclusion; guide was not written")
		}
		if ignored, err := git.Ignored(exclusion); err != nil || !ignored {
			return InstallPlan{}, errors.New("repository ignore rules override the local exclusion (for example a negation pattern); guide was not written")
		}
	}
	if err := files.MkdirAll(filepath.Dir(plan.Path), 0755); err != nil {
		return InstallPlan{}, errors.New("cannot create guide directory")
	}
	if err := files.WriteExclusive(plan.Path, []byte(Content()), 0644); err != nil {
		return InstallPlan{}, errors.New("cannot write guide; target may already exist")
	}
	return plan, nil
}
