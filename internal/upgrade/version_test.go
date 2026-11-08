package upgrade

import (
	"errors"
	"testing"
)

func TestParseVersionAcceptsPublishedReleaseTags(t *testing.T) {
	cases := []struct {
		raw  string
		want Version
	}{
		{"v0.5.0", Version{Major: 0, Minor: 5, Patch: 0}},
		{"0.5.0", Version{Major: 0, Minor: 5, Patch: 0}},
		{"v10.20.30", Version{Major: 10, Minor: 20, Patch: 30}},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := ParseVersion(tc.raw)
			if err != nil {
				t.Fatalf("ParseVersion(%q) returned unexpected error: %v", tc.raw, err)
			}
			if got.Major != tc.want.Major || got.Minor != tc.want.Minor || got.Patch != tc.want.Patch {
				t.Fatalf("ParseVersion(%q) = %d.%d.%d, want %d.%d.%d",
					tc.raw, got.Major, got.Minor, got.Patch, tc.want.Major, tc.want.Minor, tc.want.Patch)
			}
		})
	}
}

func TestParseVersionRejectsUncomparableMarkers(t *testing.T) {
	cases := []string{
		"",
		"dev",
		"v1.2",
		"v1.2.3.4",
		"v1.2.x",
		"v1.2.3-rc1",
		"v1.2.3+build",
		"V1.2.3",
		" v1.2.3",
		"v1.2.3 ",
		"v-1.2.3",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseVersion(raw); !errors.Is(err, ErrNotComparable) {
				t.Fatalf("ParseVersion(%q) error = %v, want ErrNotComparable", raw, err)
			}
		})
	}
}

func TestCompareOrdersReleaseVersions(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"v0.5.0", "v0.5.0", 0},
		{"v0.6.0", "v0.5.9", 1},
		{"v0.5.9", "v0.6.0", -1},
		{"v2.0.0", "v1.99.99", 1},
		{"v1.2.10", "v1.2.9", 1},
	}
	for _, tc := range cases {
		t.Run(tc.left+"_"+tc.right, func(t *testing.T) {
			left, err := ParseVersion(tc.left)
			if err != nil {
				t.Fatalf("ParseVersion(%q): %v", tc.left, err)
			}
			right, err := ParseVersion(tc.right)
			if err != nil {
				t.Fatalf("ParseVersion(%q): %v", tc.right, err)
			}
			if got := left.Compare(right); got != tc.want {
				t.Fatalf("%s.Compare(%s) = %d, want %d", tc.left, tc.right, got, tc.want)
			}
		})
	}
}

func TestNewerThanComparesPublishedReleases(t *testing.T) {
	cases := []struct {
		name          string
		current       string
		latest        string
		wantNewer     bool
		wantComparble bool
	}{
		{name: "newer available", current: "v0.5.0", latest: "v0.6.0", wantNewer: true, wantComparble: true},
		{name: "equal", current: "v0.6.0", latest: "v0.6.0", wantNewer: false, wantComparble: true},
		{name: "local ahead", current: "v0.7.0", latest: "v0.6.0", wantNewer: false, wantComparble: true},
		{name: "development build", current: "dev", latest: "v0.6.0", wantNewer: false, wantComparble: false},
		{name: "snapshot build", current: "v0.6.1-next", latest: "v0.6.0", wantNewer: false, wantComparble: false},
		{name: "unparseable latest", current: "v0.5.0", latest: "nightly", wantNewer: false, wantComparble: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newer, err := NewerThan(tc.current, tc.latest)
			if !tc.wantComparble {
				if !errors.Is(err, ErrNotComparable) {
					t.Fatalf("NewerThan(%q, %q) error = %v, want ErrNotComparable", tc.current, tc.latest, err)
				}
				if newer {
					t.Fatalf("NewerThan(%q, %q) = true, want false for an uncomparable build", tc.current, tc.latest)
				}
				return
			}
			if tc.current == "v0.5.0" && tc.latest == "nightly" {
				if err == nil {
					t.Fatalf("NewerThan(%q, %q) accepted an uncomparable latest tag", tc.current, tc.latest)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewerThan(%q, %q) returned unexpected error: %v", tc.current, tc.latest, err)
			}
			if newer != tc.wantNewer {
				t.Fatalf("NewerThan(%q, %q) = %v, want %v", tc.current, tc.latest, newer, tc.wantNewer)
			}
		})
	}
}
