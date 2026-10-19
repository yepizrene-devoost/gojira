package agentguide

import (
	"errors"
	"path/filepath"
	"strings"
)

// InstallPlan describes a validated destination without performing I/O.
type InstallPlan struct {
	Path      string
	Size      int
	Exclusion string
}

// Plan validates facts supplied by the caller without accessing the filesystem.
func Plan(path string, exists, directory bool) (InstallPlan, error) {
	if strings.TrimSpace(path) == "" {
		return InstallPlan{}, errors.New("guide path is required")
	}
	if exists && !directory {
		return InstallPlan{}, errors.New("guide target already exists; refusing to overwrite")
	}
	if directory {
		return InstallPlan{}, errors.New("guide target is a directory")
	}
	name := filepath.Base(filepath.Clean(path))
	if strings.EqualFold(name, "AGENTS.md") || strings.EqualFold(name, "CLAUDE.md") {
		return InstallPlan{}, errors.New("reserved guide filename is not allowed")
	}
	return InstallPlan{Path: path, Size: len(Content())}, nil
}
