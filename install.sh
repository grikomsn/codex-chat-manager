#!/usr/bin/env bash
set -euo pipefail

REPO="${REPO:-grikomsn/codex-chat-manager}"
BINARY_NAME="${BINARY_NAME:-codex-chat-manager}"
INSTALL_DIR="${INSTALL_DIR:-}"

usage() {
  cat <<'EOF'
Usage: install.sh

Environment variables:
  REPO          GitHub repository slug (default: grikomsn/codex-chat-manager)
  BINARY_NAME   Installed binary name (default: codex-chat-manager)
  INSTALL_DIR   Install destination override
EOF
}

log() {
  printf '%s\n' "$*"
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

need_cmd curl
need_cmd tar
need_cmd install
need_cmd python3

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  linux|darwin) ;;
  *) die "unsupported operating system: $os" ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) die "unsupported architecture: $arch" ;;
esac

release_api="https://api.github.com/repos/${REPO}/releases/latest"

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

download() {
  local url="$1"
  local out="$2"
  curl -fsSL "$url" -o "$out"
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    die "required command not found: sha256sum or shasum"
  fi
}

release_json="${tmpdir}/release.json"
download "$release_api" "$release_json"

release_info="$(
  python3 - "$release_json" "$os" "$arch" "$BINARY_NAME" <<'PY'
import json
import sys

path, os_name, arch, binary_name = sys.argv[1:5]
with open(path, "r", encoding="utf-8") as fh:
    release = json.load(fh)

tag = release.get("tag_name", "")
if not tag:
    raise SystemExit("latest release did not include a tag name")

asset_name = f"{binary_name}_{tag}_{os_name}_{arch}.tar.gz"
asset_url = ""
checksum_url = ""

for asset in release.get("assets", []):
    name = asset.get("name", "")
    if name == asset_name:
        asset_url = asset.get("browser_download_url", "")
    elif name == "checksums.txt":
        checksum_url = asset.get("browser_download_url", "")

print(tag)
print(asset_name)
print(asset_url)
print(checksum_url)
PY
)"

tag_name="$(printf '%s\n' "$release_info" | sed -n '1p')"
asset_name="$(printf '%s\n' "$release_info" | sed -n '2p')"
asset_url="$(printf '%s\n' "$release_info" | sed -n '3p')"
checksum_url="$(printf '%s\n' "$release_info" | sed -n '4p')"

if [[ -z "$tag_name" || -z "$asset_name" || -z "$asset_url" ]]; then
  die "could not resolve the latest release asset for ${os}/${arch}"
fi

install_dir_from_candidates() {
  if [[ -n "$INSTALL_DIR" ]]; then
    printf '%s\n' "$INSTALL_DIR"
    return
  fi

  local primary="${HOME}/.local/bin"
  local fallback="${HOME}/bin"

  mkdir -p "$primary" 2>/dev/null && [[ -w "$primary" ]] && {
    printf '%s\n' "$primary"
    return
  }

  mkdir -p "$fallback" 2>/dev/null && [[ -w "$fallback" ]] && {
    printf '%s\n' "$fallback"
    return
  }

  die "could not create a writable install directory under ~/.local/bin or ~/bin"
}

install_dir="$(install_dir_from_candidates)"
mkdir -p "$install_dir"

archive_path="${tmpdir}/${asset_name}"
log "downloading ${asset_name}"
download "$asset_url" "$archive_path"

checksum_file=""
if [[ -n "$checksum_url" ]]; then
  candidate="${tmpdir}/$(basename "$checksum_url")"
  if curl -fsSL "$checksum_url" -o "$candidate"; then
    checksum_file="$candidate"
  fi
fi

if [[ -n "$checksum_file" ]]; then
  expected="$(awk -v asset="$asset_name" '$2 == asset || $2 == "*" asset { print $1; exit }' "$checksum_file" || true)"
  if [[ -z "$expected" ]]; then
    die "checksum file was downloaded but did not contain an entry for ${asset_name}"
  fi

  actual="$(sha256_file "$archive_path")"
  if [[ "$actual" != "$expected" ]]; then
    die "checksum mismatch for ${asset_name}"
  fi
  log "verified ${asset_name} via ${checksum_file##*/}"
else
  log "checksum file not available; continuing without verification"
fi

extract_dir="${tmpdir}/extract"
mkdir -p "$extract_dir"
tar -xzf "$archive_path" -C "$extract_dir"

binary_path="$(find "$extract_dir" -type f \( -name "$BINARY_NAME" -o -name "${BINARY_NAME}.exe" \) | head -n 1 || true)"
if [[ -z "$binary_path" ]]; then
  die "could not find ${BINARY_NAME} in the release archive"
fi

target_path="${install_dir}/${BINARY_NAME}"
install -m 0755 "$binary_path" "$target_path"

log "installed ${BINARY_NAME} to ${target_path}"

case ":${PATH:-}:" in
  *":${install_dir}:"*) ;;
  *)
    log "add ${install_dir} to your PATH if it is not already there"
    ;;
esac
