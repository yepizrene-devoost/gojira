#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
tmp_root=${TMPDIR:-/tmp}/gojira-release-contract.$$
trap 'chmod -R u+rw "$tmp_root" 2>/dev/null || true; rm -rf "$tmp_root"' EXIT HUP INT TERM
mkdir -p "$tmp_root/bin"
cp "$repo_root/Makefile" "$tmp_root/Makefile"

fail() {
  printf '%s\n' "release contract test failed: $*" >&2
  exit 1
}

run_make() {
  (cd "$tmp_root" && make --no-print-directory "$@")
}

cat >"$tmp_root/CHANGELOG.md" <<'EOF'
# Changelog

## 📦 v1.2.3 – Current

First release line.

- Preserves formatting.

## 📦 v1.2.2 – Previous

Old release line.
EOF

run_make release-notes VERSION=v1.2.3 >/dev/null
cat >"$tmp_root/expected-notes" <<'EOF'

First release line.

- Preserves formatting.

EOF
cmp "$tmp_root/expected-notes" "$tmp_root/RELEASE_NOTES.md" || fail "notes were not extracted byte-for-byte"

cat >"$tmp_root/CHANGELOG.md" <<'EOF'
## 📦 v1.2.3-rc.1 – Candidate

Prerelease notes.
EOF
run_make release-notes VERSION=v1.2.3-rc.1 >/dev/null
grep -q 'Prerelease notes.' "$tmp_root/RELEASE_NOTES.md" || fail "prerelease tag was rejected"

assert_notes_rejected() {
  name=$1
  version=$2
  printf '%s\n' stale >"$tmp_root/RELEASE_NOTES.md"
  if run_make release-notes VERSION="$version" >"$tmp_root/$name.out" 2>&1; then
    fail "$name unexpectedly succeeded"
  fi
  test ! -e "$tmp_root/RELEASE_NOTES.md" || fail "$name retained release notes"
  test ! -e "$tmp_root/RELEASE_NOTES.md.tmp" || fail "$name retained temporary notes"
}

cat >"$tmp_root/CHANGELOG.md" <<'EOF'
## 📦 v1.2.4 – Wrong version

Notes.
EOF
assert_notes_rejected mismatched-tag v1.2.3

cat >"$tmp_root/CHANGELOG.md" <<'EOF'
## 📦 1.2.3 – Missing prefix

Notes.
EOF
assert_notes_rejected malformed-tag 1.2.3

cat >"$tmp_root/CHANGELOG.md" <<'EOF'
## 📦 v1.2.3 – Empty

## 📦 v1.2.2 – Previous
Old notes.
EOF
assert_notes_rejected empty-section v1.2.3

cat >"$tmp_root/bin/goreleaser" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >"$FAKE_GORELEASER_LOG"
EOF
chmod +x "$tmp_root/bin/goreleaser"
export PATH="$tmp_root/bin:$PATH"
export FAKE_GORELEASER_LOG="$tmp_root/goreleaser.log"

cat >"$tmp_root/CHANGELOG.md" <<'EOF'
## 📦 v1.2.3 – Release

Release notes.
EOF

(
  cd "$tmp_root"
  git init -q
  git config user.name 'Release Contract Fixture'
  git config user.email 'release-contract@example.invalid'
  git add Makefile CHANGELOG.md
  git commit -q -m 'fixture: initialize release contract'
)

assert_release_rejected() {
  name=$1
  shift
  : >"$tmp_root/$name.out"
  if run_make release "$@" >"$tmp_root/$name.out" 2>&1; then
    fail "$name unexpectedly succeeded"
  fi
  test ! -e "$FAKE_GORELEASER_LOG" || fail "$name invoked goreleaser"
}

rm -f "$tmp_root/.env" "$FAKE_GORELEASER_LOG"
assert_release_rejected missing-env VERSION=v1.2.3

: >"$tmp_root/.env"
chmod 000 "$tmp_root/.env"
assert_release_rejected unreadable-env VERSION=v1.2.3
chmod 600 "$tmp_root/.env"

printf '%s\n' 'OTHER=value' >"$tmp_root/.env"
assert_release_rejected missing-token VERSION=v1.2.3

printf '%s\n' 'GITHUB_TOKEN=fixture-token' >"$tmp_root/.env"
assert_release_rejected development-version VERSION=dev

assert_release_rejected untagged-version-override VERSION=v1.2.3
grep -q 'must be an exact local tag on HEAD' "$tmp_root/untagged-version-override.out" || fail "untagged override did not report the tag guard"

(cd "$tmp_root" && git tag v1.2.4)
assert_release_rejected mismatched-version-override VERSION=v1.2.3
grep -q 'must be an exact local tag on HEAD' "$tmp_root/mismatched-version-override.out" || fail "mismatched override did not report the tag guard"

(
  cd "$tmp_root"
  git commit -q --allow-empty -m 'fixture: prepare matching release tag'
  git tag v1.2.3
)
run_make release >/dev/null
test "$(cat "$FAKE_GORELEASER_LOG")" = 'release --clean --release-notes=RELEASE_NOTES.md' || fail "unexpected goreleaser arguments"
if grep -R 'fixture-token' "$tmp_root" --exclude=.env >/dev/null 2>&1; then
  fail "token was printed or copied"
fi

printf '%s\n' 'release contract checks passed'
