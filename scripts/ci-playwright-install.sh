#!/usr/bin/env bash
set -euo pipefail

apt_lock_files=(
  /var/lib/apt/lists/lock
  /var/cache/apt/archives/lock
  /var/lib/dpkg/lock-frontend
  /var/lib/dpkg/lock
)

apt_lock_pids() {
  local lock
  for lock in "${apt_lock_files[@]}"; do
    [ -e "$lock" ] || continue
    sudo fuser "$lock" 2>/dev/null || true
  done | tr ' ' '\n' | sed '/^$/d' | sort -u
}

wait_for_apt_quiescence() {
  local wait_seconds="$1"
  local deadline=$((SECONDS + wait_seconds))
  local -a pids=()

  while true; do
    mapfile -t pids < <(apt_lock_pids)
    if [ "${#pids[@]}" -eq 0 ]; then
      return 0
    fi
    if (( SECONDS >= deadline )); then
      echo "apt/dpkg locks are still held after ${wait_seconds}s by: ${pids[*]}" >&2
      return 1
    fi
    sleep 2
  done
}

terminate_stale_apt_update() {
  local -a pids=()
  local pid args
  local terminated=0
  mapfile -t pids < <(sudo fuser /var/lib/apt/lists/lock 2>/dev/null | tr ' ' '\n' | sed '/^$/d' | sort -u)

  for pid in "${pids[@]}"; do
    args="$(ps -p "$pid" -o args= 2>/dev/null || true)"
    if [[ "$args" == *"apt-get"* && "$args" == *"update"* ]]; then
      echo "terminating stale apt-get update process ${pid} before Playwright retry" >&2
      sudo kill -TERM "$pid" 2>/dev/null || true
      terminated=1
    else
      echo "refusing to terminate apt lock holder ${pid}: ${args:-unknown process}" >&2
    fi
  done

  [ "$terminated" -eq 1 ]
}

recover_apt_after_failed_playwright_install() {
  if ! command -v fuser >/dev/null 2>&1; then
    echo "cannot safely retry Playwright dependency installation because fuser is unavailable" >&2
    return 1
  fi

  if wait_for_apt_quiescence 60; then
    return 0
  fi

  if terminate_stale_apt_update; then
    if wait_for_apt_quiescence 30; then
      return 0
    fi
  fi

  echo "package manager did not quiesce safely after failed Playwright dependency install" >&2
  return 1
}

for attempt in 1 2; do
  if timeout --signal=TERM --kill-after=30s 12m npx playwright install --with-deps chromium; then
    exit 0
  fi
  if [ "$attempt" -eq 2 ]; then
    echo "Playwright Chromium dependency installation failed after two bounded attempts" >&2
    exit 1
  fi
  echo "Playwright dependency install attempt ${attempt}/2 failed; waiting for apt/dpkg cleanup before retry" >&2
  recover_apt_after_failed_playwright_install
  sleep 5
done
