#!/bin/sh
# Install GoJira on Linux or macOS without sudo.
set -eu

REPOSITORY="yepizrene-devoost/gojira"
PROJECT="gojira"
RELEASES_URL="https://github.com/${REPOSITORY}/releases"
LATEST_URL="${RELEASES_URL}/latest"
BINARY="gojira"
TMP_DIR=""
STAGE_FILE=""
DOWNLOADER=""
SHA_TOOL=""
OS=""
ARCH=""
TAG=""
VERSION=""
INSTALL_DIR=""

info() { printf '  %s\n' "$*"; }
fail() { printf 'gojira installer: %s\n' "$*" >&2; exit 1; }

usage() {
    cat <<'EOF'
GoJira installer for Linux and macOS

Usage:
  ./install.sh [--help]

Environment variables:
  GOJIRA_VERSION      Release to install, such as 1.2.3 or v1.2.3.
                      Defaults to the latest GitHub release.
  GOJIRA_INSTALL_DIR  Destination directory. Defaults to $HOME/.local/bin.
EOF
}

cleanup() {
    if [ -n "$STAGE_FILE" ] && { [ -e "$STAGE_FILE" ] || [ -L "$STAGE_FILE" ]; }; then
        rm -f "$STAGE_FILE"
    fi
    if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
        rm -rf "$TMP_DIR"
    fi
}
trap cleanup EXIT
trap 'exit 1' INT TERM HUP

parse_args() {
    for arg in "$@"; do
        case "$arg" in
            -h|--help) usage; exit 0 ;;
            *) fail "unknown option: $arg (try --help)" ;;
        esac
    done
}

detect_platform() {
    case "$(uname -s)" in
        Linux) OS="Linux" ;;
        Darwin) OS="Darwin" ;;
        MINGW*|MSYS*|CYGWIN*|Windows_NT)
            fail "use install.ps1 on Windows"
            ;;
        *) fail "unsupported operating system: $(uname -s)" ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64) ARCH="x86_64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *) fail "unsupported architecture: $(uname -m)" ;;
    esac
}

detect_tools() {
    if command -v curl >/dev/null 2>&1; then
        DOWNLOADER="curl"
    elif command -v wget >/dev/null 2>&1; then
        DOWNLOADER="wget"
    else
        fail "curl or wget is required"
    fi

    if command -v sha256sum >/dev/null 2>&1; then
        SHA_TOOL="sha256sum"
    elif command -v shasum >/dev/null 2>&1; then
        SHA_TOOL="shasum"
    else
        fail "sha256sum or shasum is required"
    fi

    command -v tar >/dev/null 2>&1 || fail "tar is required"
}

resolve_install_dir() {
    if [ -n "${GOJIRA_INSTALL_DIR:-}" ]; then
        INSTALL_DIR="$GOJIRA_INSTALL_DIR"
    elif [ -n "${HOME:-}" ]; then
        INSTALL_DIR="$HOME/.local/bin"
    else
        fail "HOME is unset; set GOJIRA_INSTALL_DIR"
    fi
    case "$INSTALL_DIR" in
        /*) ;;
        *) INSTALL_DIR="$(pwd)/$INSTALL_DIR" ;;
    esac
    case "$INSTALL_DIR" in
        */../*|*/..|*/./*|*/.) fail "install directory must not contain . or .. path components" ;;
    esac
}

prepare_install_dir() {
    [ ! -L "$INSTALL_DIR" ] || fail "install directory must not be a symlink: $INSTALL_DIR"
    if [ -e "$INSTALL_DIR" ] && [ ! -d "$INSTALL_DIR" ]; then
        fail "install directory path is not a directory: $INSTALL_DIR"
    fi
    mkdir -p "$INSTALL_DIR" || fail "could not create $INSTALL_DIR"
    [ -d "$INSTALL_DIR" ] && [ ! -L "$INSTALL_DIR" ] || fail "install directory is not a safe directory: $INSTALL_DIR"
    INSTALL_DIR="$(CDPATH= cd -- "$INSTALL_DIR" && pwd -P)" || fail "could not resolve $INSTALL_DIR"
}

download() {
    url="$1"
    destination="$2"
    if [ "$DOWNLOADER" = "curl" ]; then
        curl -fsSL -o "$destination" "$url"
    else
        wget -qO "$destination" "$url"
    fi
}

resolve_latest_tag() {
    if [ "$DOWNLOADER" = "curl" ]; then
        final_url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$LATEST_URL")" || return 1
    else
        final_url="$(wget -qS --spider "$LATEST_URL" 2>&1 | awk 'tolower($1) == "location:" { value=$2 } END { gsub("\\r", "", value); print value }')"
    fi
    [ -n "$final_url" ] || return 1
    printf '%s\n' "${final_url%/}" | awk -F/ '{ print $NF }'
}

validate_tag() {
    printf '%s\n' "$1" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$'
}

compute_sha256() {
    if [ "$SHA_TOOL" = "sha256sum" ]; then
        sha256sum "$1" | awk '{ print tolower($1) }'
    else
        shasum -a 256 "$1" | awk '{ print tolower($1) }'
    fi
}

read_expected_checksum() {
    checksum_file="$1"
    asset_name="$2"
    awk -v wanted="$asset_name" '
        NF >= 2 {
            name=$2
            sub(/^\*/, "", name)
            if (name == wanted) {
                references++
                if (NF == 2 && length($1) == 64 && $1 ~ /^[0-9A-Fa-f]+$/) {
                    valid++
                    hash=tolower($1)
                }
            }
        }
        END {
            if (references != 1 || valid != 1) exit 1
            print hash
        }
    ' "$checksum_file"
}

validate_archive() {
    archive="$1"
    listing="$TMP_DIR/archive.list"
    verbose_listing="$TMP_DIR/archive.verbose.list"
    tar -tzf "$archive" >"$listing" || fail "could not inspect $ASSET"
    tar -tvzf "$archive" >"$verbose_listing" || fail "could not inspect entry types in $ASSET"
    awk '
        /^\// || /(^|\/)\.\.($|\/)/ { unsafe=1 }
        $0 == "gojira" { binary++ }
        $0 == "gojira.exe" || /(^|\/)gojira$/ && $0 != "gojira" { unexpected=1 }
        END { if (unsafe || binary != 1 || unexpected) exit 1 }
    ' "$listing" || fail "archive must contain one safe root gojira binary"
    awk '
        substr($0, 1, 1) != "-" && substr($0, 1, 1) != "d" { exit 1 }
    ' "$verbose_listing" || fail "archive contains links or unsupported entry types"
}

main() {
    parse_args "$@"
    detect_platform
    detect_tools
    resolve_install_dir

    if [ -n "${GOJIRA_VERSION:-}" ]; then
        case "$GOJIRA_VERSION" in
            v*) TAG="$GOJIRA_VERSION" ;;
            *) TAG="v$GOJIRA_VERSION" ;;
        esac
    else
        info "Resolving the latest GoJira release"
        TAG="$(resolve_latest_tag)" || fail "could not resolve the latest release; set GOJIRA_VERSION"
    fi
    validate_tag "$TAG" || fail "invalid release version: $TAG"

    VERSION="${TAG#v}"
    ASSET="${PROJECT}_${OS}_${ARCH}.tar.gz"
    CHECKSUMS="checksums.txt"
    BASE_URL="${RELEASES_URL}/download/${TAG}"
    TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gojira-install.XXXXXX")" || fail "could not create a temporary directory"

    info "Downloading $ASSET"
    download "$BASE_URL/$ASSET" "$TMP_DIR/$ASSET" || fail "could not download $ASSET"
    download "$BASE_URL/$CHECKSUMS" "$TMP_DIR/$CHECKSUMS" || fail "could not download $CHECKSUMS"

    if ! EXPECTED="$(read_expected_checksum "$TMP_DIR/$CHECKSUMS" "$ASSET")"; then
        fail "checksums.txt must contain one valid SHA-256 entry for $ASSET"
    fi
    ACTUAL="$(compute_sha256 "$TMP_DIR/$ASSET")"
    [ "$EXPECTED" = "$ACTUAL" ] || fail "checksum mismatch for $ASSET"
    info "Verified SHA-256 checksum"

    validate_archive "$TMP_DIR/$ASSET"
    mkdir "$TMP_DIR/extract" || fail "could not prepare extraction directory"
    tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR/extract" || fail "could not extract $ASSET"
    [ -f "$TMP_DIR/extract/$BINARY" ] || fail "archive did not contain $BINARY"

    prepare_install_dir
    target="$INSTALL_DIR/$BINARY"
    [ ! -L "$target" ] || fail "refusing to replace symlink destination: $target"
    if [ -e "$target" ] && [ ! -f "$target" ]; then
        fail "destination is not a regular file: $target"
    fi
    STAGE_FILE="$(mktemp "$INSTALL_DIR/.$BINARY.new.XXXXXX")" || fail "could not create a safe staging file in $INSTALL_DIR"
    cp "$TMP_DIR/extract/$BINARY" "$STAGE_FILE" || fail "could not stage $BINARY in $INSTALL_DIR"
    chmod 755 "$STAGE_FILE" || fail "could not make staged binary executable"
    mv -f "$STAGE_FILE" "$target" || fail "could not replace $target"
    STAGE_FILE=""

    printf 'GoJira %s installed at %s\n' "$VERSION" "$INSTALL_DIR/$BINARY"
    case ":${PATH:-}:" in
        *":$INSTALL_DIR:"*) printf "Run 'gojira --help' to get started.\n" ;;
        *)
            printf '%s is not on PATH. Add this line to your shell profile:\n' "$INSTALL_DIR"
            printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
            ;;
    esac
}

main "$@"
