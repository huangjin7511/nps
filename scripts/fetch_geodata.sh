#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONF_DIR="${CONF_DIR:-$ROOT_DIR/conf}"
GEODATA_MODE="${GEODATA_MODE:-always}"
GEOIP_URL="${GEOIP_URL:-https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat}"
GEOSITE_URL="${GEOSITE_URL:-https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat}"

log() {
  printf '==> %s\n' "$*"
}

pick_downloader() {
  if command -v curl >/dev/null 2>&1; then
    echo "curl"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    echo "wget"
    return
  fi
  printf 'Missing downloader: require curl or wget\n' >&2
  exit 1
}

download_file() {
  local downloader="$1"
  local url="$2"
  local dest="$3"
  local tmp="${dest}.tmp.$$"

  rm -f "$tmp"
  case "$downloader" in
    curl)
      curl -fL --retry 5 --retry-delay 2 --retry-all-errors --connect-timeout 20 -o "$tmp" "$url"
      ;;
    wget)
      wget -qO "$tmp" "$url"
      ;;
    *)
      printf 'Unsupported downloader: %s\n' "$downloader" >&2
      exit 1
      ;;
  esac

  if [[ ! -s "$tmp" ]]; then
    rm -f "$tmp"
    printf 'Downloaded file is empty: %s\n' "$url" >&2
    exit 1
  fi
  mv "$tmp" "$dest"
}

fetch_one() {
  local downloader="$1"
  local url="$2"
  local dest="$3"

  if [[ "$GEODATA_MODE" == "if_missing" && -s "$dest" ]]; then
    log "Keeping existing $(basename "$dest")"
    return
  fi

  log "Downloading $(basename "$dest")"
  download_file "$downloader" "$url" "$dest"
}

main() {
  case "$GEODATA_MODE" in
    always|if_missing)
      ;;
    off)
      log "Skipping geodata download"
      exit 0
      ;;
    *)
      printf 'Invalid GEODATA_MODE: %s\n' "$GEODATA_MODE" >&2
      exit 1
      ;;
  esac

  mkdir -p "$CONF_DIR"
  local downloader
  downloader="$(pick_downloader)"
  fetch_one "$downloader" "$GEOIP_URL" "$CONF_DIR/geoip.dat"
  fetch_one "$downloader" "$GEOSITE_URL" "$CONF_DIR/geosite.dat"
}

main "$@"
