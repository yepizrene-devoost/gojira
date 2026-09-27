#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
TMP=$(mktemp -d "${TMPDIR:-/tmp}/gojira-build-all-test.XXXXXX")
trap 'rm -rf "$TMP"' EXIT INT TERM HUP
BIN_DIR="$TMP/bin"
mkdir -p "$BIN_DIR"
printf 'preserve me\n' >"$BIN_DIR/unrelated"

fail() {
    printf '%s\n' "build-all contract test failed: $*" >&2
    exit 1
}

(
    cd "$ROOT"
    GOPROXY=off GOTOOLCHAIN=local \
        make --no-print-directory build-all BIN_DIR="$BIN_DIR" VERSION=contract-test
)

assert_artifact() {
    os=$1
    arch=$2
    suffix=$3
    artifact="$BIN_DIR/gojira-$os-$arch$suffix"

    test -f "$artifact" || fail "missing $artifact"
    build_info=$(go version -m "$artifact") || fail "cannot read Go build info from $artifact"
    printf '%s\n' "$build_info" | grep -Fq "GOOS=$os" || fail "$artifact does not report GOOS=$os"
    printf '%s\n' "$build_info" | grep -Fq "GOARCH=$arch" || fail "$artifact does not report GOARCH=$arch"
    printf '%s\n' "$build_info" | grep -Fq 'CGO_ENABLED=0' || fail "$artifact does not report CGO_ENABLED=0"
    printf '%s\n' "$build_info" | grep -Fq -- '-X main.version=contract-test' || fail "$artifact does not contain the requested version linker flag"
}

assert_artifact linux amd64 ''
assert_artifact linux arm64 ''
assert_artifact darwin amd64 ''
assert_artifact darwin arm64 ''
assert_artifact windows amd64 '.exe'
assert_artifact windows arm64 '.exe'

test "$(cat "$BIN_DIR/unrelated")" = 'preserve me' || fail 'unrelated BIN_DIR content was changed'
test "$(find "$BIN_DIR" -type f | wc -l | tr -d ' ')" = 7 || fail 'unexpected output set'

printf 'build-all contract checks passed\n'
