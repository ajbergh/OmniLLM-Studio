#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -eq 0 ]; then
  echo "usage: $0 <apt-package> [...]" >&2
  exit 2
fi

apt_retry() {
  local attempts="$1"
  local timeout_value="$2"
  shift 2
  local attempt
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if timeout --signal=TERM --kill-after=30s "$timeout_value" \
      sudo env DEBIAN_FRONTEND=noninteractive apt-get \
        -o DPkg::Lock::Timeout=60 \
        -o Acquire::Retries=3 \
        -o Acquire::http::Timeout=30 \
        -o Acquire::https::Timeout=30 \
        "$@"; then
      return 0
    fi
    if [ "$attempt" -eq "$attempts" ]; then
      echo "apt command failed after ${attempts} bounded attempts: apt-get $*" >&2
      return 1
    fi
    echo "apt command attempt ${attempt}/${attempts} failed; retrying" >&2
    sleep $((attempt * 5))
  done
}

apt_retry 3 3m update -qq
apt_retry 2 8m install -y --no-install-recommends "$@"
