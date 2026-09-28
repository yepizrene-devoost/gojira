#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
TMP=$(mktemp -d "${TMPDIR:-/tmp}/gojira-installer-test.XXXXXX")
trap 'rm -rf "$TMP"' EXIT INT TERM HUP
FIXTURE="$TMP/fixture"
BIN="$TMP/mock-bin"
INSTALL="$TMP/install"
ASSET="gojira_Linux_x86_64.tar.gz"
mkdir -p "$FIXTURE" "$BIN" "$INSTALL"

cat >"$BIN/uname" <<'EOF'
#!/bin/sh
case "$1" in
  -s) printf 'Linux\n' ;;
  -m) printf 'x86_64\n' ;;
  *) exit 1 ;;
esac
EOF

cat >"$BIN/curl" <<'EOF'
#!/bin/sh
out=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
[ -n "$out" ] && [ -n "$url" ] || exit 90
cp "$INSTALLER_FIXTURE_DIR/${url##*/}" "$out"
EOF
REAL_MKTEMP=$(command -v mktemp)
cat >"$BIN/mktemp" <<'EOF'
#!/bin/sh
case "${1:-}" in
    */.gojira.new.XXXXXX)
        if [ -n "${INSTALLER_STAGE_COLLISION:-}" ]; then
            [ -L "$INSTALLER_STAGE_COLLISION" ] || exit 91
            exit 1
        fi
        ;;
esac
exec "$INSTALLER_REAL_MKTEMP" "$@"
EOF
chmod +x "$BIN/uname" "$BIN/curl" "$BIN/mktemp"

sha256() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{ print $1 }'
    else
        shasum -a 256 "$1" | awk '{ print $1 }'
    fi
}

make_archive() {
    content="$1"
    payload="$TMP/payload"
    rm -rf "$payload"
    mkdir -p "$payload"
    printf '%s\n' "$content" >"$payload/gojira"
    chmod +x "$payload/gojira"
    tar -czf "$FIXTURE/$ASSET" -C "$payload" gojira
    printf '%s  %s\n' "$(sha256 "$FIXTURE/$ASSET")" "$ASSET" >"$FIXTURE/checksums.txt"
}

run_installer() {
    PATH="$BIN:$PATH" \
    INSTALLER_FIXTURE_DIR="$FIXTURE" \
    INSTALLER_REAL_MKTEMP="$REAL_MKTEMP" \
    INSTALLER_STAGE_COLLISION="${INSTALLER_STAGE_COLLISION:-}" \
    GOJIRA_VERSION="v1.2.3" \
    GOJIRA_INSTALL_DIR="$INSTALL" \
    HOME="$TMP/home" \
    sh "$ROOT/scripts/install.sh"
}

make_archive "new binary"
printf 'old binary\n' >"$INSTALL/gojira"
run_installer >/dev/null
[ "$(cat "$INSTALL/gojira")" = "new binary" ] || {
    echo "installed binary did not replace the old binary" >&2
    exit 1
}
[ -x "$INSTALL/gojira" ] || { echo "installed binary is not executable" >&2; exit 1; }

printf 'old binary\n' >"$INSTALL/gojira"
asset_hash=$(sha256 "$FIXTURE/$ASSET")
printf '%s  %s\n%s  %s\n' "$asset_hash" "$ASSET" "$asset_hash" "$ASSET" >"$FIXTURE/checksums.txt"
if run_installer >"$TMP/duplicate.out" 2>&1; then
    echo "duplicate checksum entry was accepted" >&2
    exit 1
fi
[ "$(cat "$INSTALL/gojira")" = "old binary" ] || { echo "failed verification changed the installed binary" >&2; exit 1; }

printf '%s  %s\nnot-a-hash  %s  unexpected-field\n' "$asset_hash" "$ASSET" "$ASSET" >"$FIXTURE/checksums.txt"
if run_installer >"$TMP/malformed-duplicate.out" 2>&1; then
    echo "malformed duplicate checksum reference was accepted" >&2
    exit 1
fi
[ "$(cat "$INSTALL/gojira")" = "old binary" ] || { echo "malformed checksum manifest changed the installed binary" >&2; exit 1; }

printf '%064d  %s\n' 0 "$ASSET" >"$FIXTURE/checksums.txt"
if run_installer >"$TMP/mismatch.out" 2>&1; then
    echo "checksum mismatch was accepted" >&2
    exit 1
fi
[ "$(cat "$INSTALL/gojira")" = "old binary" ] || { echo "checksum mismatch changed the installed binary" >&2; exit 1; }

payload="$TMP/missing-payload"
mkdir -p "$payload"
printf 'not a binary\n' >"$payload/README.txt"
tar -czf "$FIXTURE/$ASSET" -C "$payload" README.txt
printf '%s  %s\n' "$(sha256 "$FIXTURE/$ASSET")" "$ASSET" >"$FIXTURE/checksums.txt"
if run_installer >"$TMP/missing.out" 2>&1; then
    echo "archive without gojira was accepted" >&2
    exit 1
fi
[ "$(cat "$INSTALL/gojira")" = "old binary" ] || { echo "missing binary archive changed the installed binary" >&2; exit 1; }

payload="$TMP/link-payload"
rm -rf "$payload"
mkdir -p "$payload"
printf 'outside sentinel\n' >"$TMP/archive-symlink-outside"
ln -s "$TMP/archive-symlink-outside" "$payload/gojira"
tar -czf "$FIXTURE/$ASSET" -C "$payload" gojira
printf '%s  %s\n' "$(sha256 "$FIXTURE/$ASSET")" "$ASSET" >"$FIXTURE/checksums.txt"
if run_installer >"$TMP/symlink-archive.out" 2>&1; then
    echo "archive symlink was accepted" >&2
    exit 1
fi
[ "$(cat "$TMP/archive-symlink-outside")" = "outside sentinel" ] || {
    echo "archive symlink caused an unsafe extraction side effect" >&2
    exit 1
}
[ "$(cat "$INSTALL/gojira")" = "old binary" ] || {
    echo "archive symlink changed the installed binary" >&2
    exit 1
}

rm -rf "$payload"
mkdir -p "$payload"
printf 'linked binary\n' >"$payload/source"
ln "$payload/source" "$payload/gojira"
tar -czf "$FIXTURE/$ASSET" -C "$payload" source gojira
printf '%s  %s\n' "$(sha256 "$FIXTURE/$ASSET")" "$ASSET" >"$FIXTURE/checksums.txt"
if run_installer >"$TMP/hardlink-archive.out" 2>&1; then
    echo "archive hardlink was accepted" >&2
    exit 1
fi

rm -rf "$payload"
mkdir -p "$payload"
printf 'new binary\n' >"$payload/gojira"
mkfifo "$payload/unsafe-pipe"
tar -czf "$FIXTURE/$ASSET" -C "$payload" gojira unsafe-pipe
printf '%s  %s\n' "$(sha256 "$FIXTURE/$ASSET")" "$ASSET" >"$FIXTURE/checksums.txt"
if run_installer >"$TMP/unsafe-type.out" 2>&1; then
    echo "archive special file was accepted" >&2
    exit 1
fi

rm -rf "$payload"
mkdir -p "$payload"
printf 'new binary\n' >"$payload/gojira"
printf 'outside\n' >"$TMP/absolute-entry"
tar -czPf "$FIXTURE/$ASSET" -C "$payload" gojira "$TMP/absolute-entry"
printf '%s  %s\n' "$(sha256 "$FIXTURE/$ASSET")" "$ASSET" >"$FIXTURE/checksums.txt"
if run_installer >"$TMP/unsafe-path.out" 2>&1; then
    echo "archive absolute path was accepted" >&2
    exit 1
fi

make_archive "new binary"
printf 'victim\n' >"$TMP/victim"
rm -f "$INSTALL/gojira"
ln -s "$TMP/victim" "$INSTALL/gojira"
if run_installer >"$TMP/destination-symlink.out" 2>&1; then
    echo "destination symlink was accepted" >&2
    exit 1
fi
[ "$(cat "$TMP/victim")" = "victim" ] || { echo "destination symlink target was changed" >&2; exit 1; }

rm -f "$INSTALL/gojira"
printf 'staging victim\n' >"$TMP/staging-victim"
STAGE_COLLISION="$INSTALL/.gojira.new.COLLIDE"
ln -s "$TMP/staging-victim" "$STAGE_COLLISION"
if INSTALLER_STAGE_COLLISION="$STAGE_COLLISION" run_installer >"$TMP/staging-symlink.out" 2>&1; then
    echo "staging symlink collision was accepted" >&2
    exit 1
fi
[ "$(cat "$TMP/staging-victim")" = "staging victim" ] || {
    echo "staging symlink target was changed" >&2
    exit 1
}
[ -L "$STAGE_COLLISION" ] || { echo "staging symlink collision was removed" >&2; exit 1; }
rm -f "$STAGE_COLLISION"

mkdir "$INSTALL/gojira"
if run_installer >"$TMP/destination-directory.out" 2>&1; then
    echo "destination directory was accepted" >&2
    exit 1
fi
[ ! -e "$INSTALL/gojira/gojira" ] || { echo "installer moved the binary into the destination directory" >&2; exit 1; }
rmdir "$INSTALL/gojira"

if PATH="$BIN:$PATH" INSTALLER_FIXTURE_DIR="$FIXTURE" GOJIRA_VERSION='../bad' \
    GOJIRA_INSTALL_DIR="$INSTALL" HOME="$TMP/home" sh "$ROOT/scripts/install.sh" >"$TMP/version.out" 2>&1; then
    echo "malformed version was accepted" >&2
    exit 1
fi

# The release contract must keep six stable names and publish both installers.
grep -Fq '{{ .ProjectName }}_{{ title .Os }}_{{ if eq .Arch "amd64" }}x86_64{{ else }}{{ .Arch }}{{ end }}' "$ROOT/.goreleaser.yaml"
grep -Fq 'scripts/install.sh' "$ROOT/.goreleaser.yaml"
grep -Fq 'scripts/install.ps1' "$ROOT/.goreleaser.yaml"

# Static PowerShell checks still run on hosts without pwsh; Windows CI executes
# the behavioral fixture separately.
POWERSHELL="$ROOT/scripts/install.ps1"
grep -Fq '${Project}_Windows_$arch.zip' "$POWERSHELL"
grep -Fq 'Get-FileHash -LiteralPath $archivePath -Algorithm SHA256' "$POWERSHELL"
grep -Fq '[System.IO.File]::Replace($staged, $Target, $backup, $true)' "$POWERSHELL"
grep -Fq "[Guid]::NewGuid().ToString('N')" "$POWERSHELL"
grep -Fq "[Environment]::SetEnvironmentVariable('Path', \$newPath, 'User')" "$POWERSHELL"
grep -Fq "'^v(0|[1-9][0-9]*)" "$POWERSHELL"
grep -Fq 'Archive must contain exactly one root $Binary binary.' "$POWERSHELL"

printf 'POSIX installer contract passed\n'
