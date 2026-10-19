package agentguide

import (
	"errors"
	"io/fs"
)

// ErrOutsideWorkingTree reports that a target is not inside a Git working tree.
// Any other Prepare error means the repository state could not be determined.
var ErrOutsideWorkingTree = errors.New("target is not inside a Git working tree")

// Filesystem is the destination inspection boundary used by the installer.
type Filesystem interface {
	Lstat(string) (fs.FileInfo, error)
	MkdirAll(string, fs.FileMode) error
	WriteExclusive(string, []byte, fs.FileMode) error
}

// Exclusion is a prepared repository exclusion, without side effects.
type Exclusion struct {
	File     string
	Line     string
	Root     string // repository root, used to ask Git about the effective result
	Relative string // slash-separated path of the target below Root
}

// GitExclude resolves and applies repository-local exclusions.
type GitExclude interface {
	Prepare(string) (Exclusion, error)
	Apply(Exclusion) error
	// Ignored reports whether Git effectively ignores the target. The local
	// exclude file has the lowest precedence, so a successful Apply is not proof.
	Ignored(Exclusion) (bool, error)
}
