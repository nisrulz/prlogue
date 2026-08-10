#!/bin/sh
set -eu

# PRlogue installer
# Detects OS/arch, downloads the latest release, verifies checksum,
# and installs to ~/go/bin.

REPO="nisrulz/prlogue"
BINARY="prlogue"
API_BASE="${PRLOGUE_INSTALL_API_BASE:-https://api.github.com/repos/${REPO}}"
DOWNLOAD_BASE="${PRLOGUE_INSTALL_DOWNLOAD_BASE:-https://github.com/${REPO}/releases/download}"

# ---------- helpers ----------

fail() { printf '\033[31m%s\033[0m\n' "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

# Show a path with $HOME replaced by ~ for display.
tilde() {
  case "$1" in
    "$HOME"*) printf '~%s' "${1#$HOME}" ;;
    *)        printf '%s' "$1" ;;
  esac
}

# Show a path with $HOME replaced by $HOME (literal) so it stays valid in a
# double-quoted export line without leaking the real home directory.
homevar() {
  case "$1" in
    "$HOME"*) printf '$HOME%s' "${1#$HOME}" ;;
    *)        printf '%s' "$1" ;;
  esac
}

# ---------- detect OS / arch ----------

detect_platform() {
  os="$(uname -s)"
  arch="$(uname -m)"

  case "$os" in
    Darwin)  os="darwin" ;;
    Linux)   os="linux"  ;;
    *)       fail "unsupported OS: $os" ;;
  esac

  case "$arch" in
    x86_64|amd64)   arch="amd64" ;;
    arm64|aarch64)  arch="arm64" ;;
    *)              fail "unsupported architecture: $arch" ;;
  esac

  OS="$os"
  ARCH="$arch"
}

# ---------- fetch latest version ----------

latest_version() {
  need curl
  url="${API_BASE}/releases/latest"
  # GitHub returns 302; curl follows by default.
  version="$(curl -sfL "$url" | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -1)"
  [ -n "$version" ] || fail "could not determine latest release from $url"
  VERSION="$version"
}

# ---------- resolve version ----------

# resolve_version sets VERSION. An optional version tag (with or without a
# leading "v") pins the install; otherwise the latest release is used.
resolve_version() {
  if [ -n "${1:-}" ]; then
    version="$1"
    version="${version#v}"
    case "$version" in
      *[!0-9A-Za-z._-]* | "") fail "invalid version tag: $1" ;;
    esac
    VERSION="$version"
    printf 'Installing %s v%s ...\n' "$BINARY" "$VERSION"
    return
  fi
  latest_version
}

# ---------- download & verify ----------

download_and_verify() {
  need curl
  need mktemp

  archive="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
  url="${DOWNLOAD_BASE}/v${VERSION}/${archive}"
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  printf 'Downloading %s ...\n' "$url"
  curl -sfL -o "${tmpdir}/${archive}" "$url" || fail "download failed: $url"

  # Verify SHA-256 checksum.
  checksum_url="${DOWNLOAD_BASE}/v${VERSION}/checksums.txt"
  printf 'Verifying checksum ...\n'
  curl -sfL -o "${tmpdir}/checksums.txt" "$checksum_url" || fail "could not download checksums"

  expected="$(awk -v file="$archive" '$2 == file {print $1; exit}' "${tmpdir}/checksums.txt")"
  [ "${#expected}" -eq 64 ] || fail "missing or invalid checksum for ${archive}"
  printf '%s' "$expected" | tr -d '0-9a-fA-F' | grep -q . && fail "checksum contains non-hex characters"

  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "${tmpdir}/${archive}" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "${tmpdir}/${archive}" | awk '{print $1}')"
  else
    fail "no sha256sum or shasum found"
  fi

  [ "$expected" = "$actual" ] || fail "checksum mismatch: expected $expected, got $actual"

  # Extract.
  tar -xzf "${tmpdir}/${archive}" -C "$tmpdir"
  BIN="$(find "$tmpdir" -maxdepth 1 -name "$BINARY" -type f | head -1)"
  [ -n "$BIN" ] || fail "binary not found in archive"
}

# ---------- install ----------

install_binary() {
  dest="${GOBIN:-$HOME/go/bin}"
  mkdir -p "$dest"
  # rm first so the install always replaces an existing binary, even a
  # read-only or currently-running one.
  rm -f "$dest/$BINARY"
  mv "$BIN" "$dest/$BINARY"
  chmod +x "$dest/$BINARY"
  printf 'Installed %s to %s/%s\n' "$BINARY" "$(tilde "$dest")" "$BINARY"
}

# ---------- PATH ----------

ensure_path() {
  dest="${GOBIN:-$HOME/go/bin}"
  export_line="export PATH=\"${dest}:\$PATH\""
  hint_line="export PATH=\"$(homevar "$dest"):\$PATH\""

  # Already on PATH?
  case ":$PATH:" in
    *":${dest}:"*) return ;;
  esac

  # Find rc file.
  rc=""
  for f in .zshrc .bashrc .bash_profile .zprofile; do
    [ -f "$HOME/$f" ] && rc="$HOME/$f" && break
  done

  if [ -n "$rc" ] && ! grep -q "$dest" "$rc" 2>/dev/null; then
    printf '\n%s\n' "$export_line" >> "$rc"
    printf 'Added %s to PATH in %s\n' "$(tilde "$dest")" "$(tilde "$rc")"
  fi

  printf '\nOpen a new terminal or run:\n  %s\n' "$hint_line"
}

# ---------- main ----------

main() {
  need uname
  detect_platform
  resolve_version "${1:-}"
  download_and_verify
  install_binary
  ensure_path
  printf '\nDone! Run "%s --help" to get started.\n' "$BINARY"
}

main "$@"
