#!/usr/bin/env bash
set +e

mkdir -p backend/cmd/desktop/frontend_dist
printf '<!doctype html><title>validation placeholder</title>' > backend/cmd/desktop/frontend_dist/index.html

(
  cd backend || exit 1
  timeout 15m go test -timeout 3m ./...
) > /tmp/backend.log 2>&1
backend_status=$?

(
  cd backend || exit 1
  timeout 15m go test -race -timeout 5m ./internal/rag ./internal/repository ./internal/filelibrary ./internal/document ./internal/api
) > /tmp/race.log 2>&1
race_status=$?

(
  cd frontend || exit 1
  timeout 15m npm ci &&
  timeout 15m npm run build &&
  timeout 15m npm run lint &&
  timeout 15m npm run test:unit
) > /tmp/frontend.log 2>&1
frontend_status=$?

git diff --check > /tmp/diff.log 2>&1
diff_status=$?
rm -rf backend/cmd/desktop/frontend_dist

label() {
  if [ "$1" -eq 0 ]; then
    printf 'PASS'
  elif [ "$1" -eq 124 ]; then
    printf 'TIMEOUT'
  else
    printf 'FAIL'
  fi
}

BACKEND_LABEL=$(label "$backend_status") \
RACE_LABEL=$(label "$race_status") \
FRONTEND_LABEL=$(label "$frontend_status") \
DIFF_LABEL=$(label "$diff_status") \
python3 - <<'PY'
from pathlib import Path
import os
import re

path = Path('docs/RAG_BACKEND_VALIDATION.md')
text = path.read_text()
table = f'''| Check | Result |
|---|---|
| `go test -timeout 3m ./...` | **{os.environ["BACKEND_LABEL"]}** |
| Focused Go race tests | **{os.environ["RACE_LABEL"]}** |
| Frontend build, lint, and unit tests | **{os.environ["FRONTEND_LABEL"]}** |
| `git diff --check` | **{os.environ["DIFF_LABEL"]}** |'''
text = re.sub(r'\| Check \| Result \|\n\|---\|---\|\n(?:\|.*\n){4}', table + '\n', text, count=1)
note = '''

## Test harness correction

The initial repository-wide backend run timed out in `internal/agent.TestExecuteToolCallAskApprovalApproved`. Both approval tests used a one-slot event channel while forwarding every post-approval tool event, so the callback could block after the only consumer exited. The test callbacks now forward only `EventApprovalRequired`, which is the event under assertion. Production agent behavior was not changed.
'''
if '## Test harness correction' not in text:
    text += note
path.write_text(text)
PY

if [ "$backend_status" -ne 0 ]; then
  echo '--- backend ---'
  tail -n 300 /tmp/backend.log
fi
if [ "$race_status" -ne 0 ]; then
  echo '--- race ---'
  tail -n 300 /tmp/race.log
fi
if [ "$frontend_status" -ne 0 ]; then
  echo '--- frontend ---'
  tail -n 300 /tmp/frontend.log
fi
if [ "$diff_status" -ne 0 ]; then
  echo '--- diff ---'
  cat /tmp/diff.log
fi

rm -f .github/workflows/rag-package-timeout-diagnostic.yml .rag-final-validation.sh
git add -A
git config user.name 'github-actions[bot]'
git config user.email '41898282+github-actions[bot]@users.noreply.github.com'
git commit -m 'docs(rag): record final branch validation [skip ci]'
git pull --rebase origin feature/rag-modernization-v2
git push origin HEAD:feature/rag-modernization-v2

if [ "$backend_status" -ne 0 ] || [ "$race_status" -ne 0 ] || [ "$frontend_status" -ne 0 ] || [ "$diff_status" -ne 0 ]; then
  exit 1
fi
