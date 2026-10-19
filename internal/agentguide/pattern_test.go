package agentguide

import "testing"

func TestExcludePatternAnchorsToRepositoryRoot(t *testing.T) {
	for _, test := range []struct{ name, relative, want string }{
		{"nested", "docs/guide.md", "/docs/guide.md"},
		{"root file is not matched in other directories", "guide.md", "/guide.md"},
		{"leading hash is not a comment once anchored", "#notes.md", "/#notes.md"},
		{"leading bang is not a negation once anchored", "!keep.md", "/!keep.md"},
		{"inner space is kept", "my guide.md", "/my guide.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := ExcludePattern(test.relative)
			if err != nil || got != test.want {
				t.Fatalf("ExcludePattern(%q) = %q, %v; want %q", test.relative, got, err, test.want)
			}
		})
	}
}

func TestExcludePatternEscapesGlobAndBackslash(t *testing.T) {
	for _, test := range []struct{ name, relative, want string }{
		{"star", "a*b.md", `/a\*b.md`},
		{"question mark", "a?b.md", `/a\?b.md`},
		{"opening bracket", "a[b.md", `/a\[b.md`},
		{"backslash", `a\b.md`, `/a\\b.md`},
		{"all together in a directory", `d*/e?[f\g.md`, `/d\*/e\?\[f\\g.md`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := ExcludePattern(test.relative)
			if err != nil || got != test.want {
				t.Fatalf("ExcludePattern(%q) = %q, %v; want %q", test.relative, got, err, test.want)
			}
		})
	}
}

func TestExcludePatternQuotesTrailingSpaces(t *testing.T) {
	for _, test := range []struct{ name, relative, want string }{
		{"one trailing space", "a.md ", `/a.md\ `},
		{"two trailing spaces", "a.md  ", `/a.md\ \ `},
		{"inner space stays plain", "a b.md", "/a b.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := ExcludePattern(test.relative)
			if err != nil || got != test.want {
				t.Fatalf("ExcludePattern(%q) = %q, %v; want %q", test.relative, got, err, test.want)
			}
		})
	}
}

func TestExcludePatternRejectsLineBreaks(t *testing.T) {
	for _, relative := range []string{"a\nb.md", "a\rb.md", "dir/\n/x.md"} {
		t.Run(relative, func(t *testing.T) {
			if got, err := ExcludePattern(relative); err == nil {
				t.Fatalf("ExcludePattern(%q) = %q; want an error because a line break would inject another pattern", relative, got)
			}
		})
	}
}
