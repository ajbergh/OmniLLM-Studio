#!/usr/bin/env bash
set -euo pipefail

for attempt in 1 2; do
  if timeout --signal=TERM --kill-after=30s 12m npx playwright install --with-deps chromium; then
    exit 0
  fi
  if [ "$attempt" -eq 2 ]; then
    echo "Playwright Chromium dependency installation failed after two bounded attempts" >&2
    exit 1
  fi
  echo "Playwright dependency install attempt ${attempt}/2 failed; retrying" >&2
  sleep 10
done
