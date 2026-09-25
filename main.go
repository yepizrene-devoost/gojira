// gojira is a terminal tool for managing Jira Cloud boards and issues,
// combining an interactive Bubble Tea TUI with a scriptable cobra CLI whose
// JSON output is designed for AI agents.
//
// The version marker is injected at build time via ldflags
// (-X main.version=$(VERSION)); development builds keep "dev". The installed
// revision is self-reported from the binary's own VCS stamp (buildvcs).
package main

import (
	"github.com/yepizrene-devoost/gojira/cmd"
)

var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
