// Package upgrade resolves the published GoJira releases and replaces an
// installer-owned gojira binary with a newer one.
package upgrade

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrNotComparable reports a version marker that is not a published release
// version, so no ordered comparison is possible.
var ErrNotComparable = errors.New("not a comparable release version")

// Version is a comparable release version, vMAJOR.MINOR.PATCH.
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion parses a published release tag. An optional leading "v" is
// accepted; pre-release, build-metadata and development markers are rejected
// with ErrNotComparable because they cannot be ordered against a release.
func ParseVersion(raw string) (Version, error) {
	body := strings.TrimPrefix(raw, "v")
	parts := strings.Split(body, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("%w: %q", ErrNotComparable, raw)
	}
	numbers := make([]int, 3)
	for index, part := range parts {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return Version{}, fmt.Errorf("%w: %q", ErrNotComparable, raw)
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, fmt.Errorf("%w: %q", ErrNotComparable, raw)
		}
		numbers[index] = value
	}
	return Version{Major: numbers[0], Minor: numbers[1], Patch: numbers[2]}, nil
}

// Compare reports -1, 0 or 1 when v is less than, equal to or greater than
// other.
func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		return clampSign(v.Major - other.Major)
	}
	if v.Minor != other.Minor {
		return clampSign(v.Minor - other.Minor)
	}
	return clampSign(v.Patch - other.Patch)
}

func clampSign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}

// NewerThan reports whether latest is a newer published release than current.
// A current marker that cannot be compared (for example a "dev" build) returns
// ErrNotComparable and is never treated as upgradable.
func NewerThan(current, latest string) (bool, error) {
	currentVersion, err := ParseVersion(current)
	if err != nil {
		return false, err
	}
	latestVersion, err := ParseVersion(latest)
	if err != nil {
		return false, fmt.Errorf("latest release tag %q: %w", latest, err)
	}
	return latestVersion.Compare(currentVersion) > 0, nil
}
