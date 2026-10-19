package agentguide

import (
	"errors"
	"strings"
)

// gitignoreEscaper quotes the characters that have a pattern meaning.
var gitignoreEscaper = strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`)

// ExcludePattern turns a repository-relative path into a gitignore line that
// matches exactly that path. It is pure: it performs no I/O.
func ExcludePattern(relative string) (string, error) {
	if strings.ContainsAny(relative, "\r\n") {
		return "", errors.New("guide path must not contain line breaks")
	}
	trimmed := strings.TrimRight(relative, " ")
	trailing := len(relative) - len(trimmed)
	return "/" + gitignoreEscaper.Replace(trimmed) + strings.Repeat(`\ `, trailing), nil
}
