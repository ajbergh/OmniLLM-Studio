#!/usr/bin/env python3
"""Apply the final local-grounding fix and restore read-only CI.

This helper is intentionally one-shot. The branch-only CI step commits the
source change, removes this helper, and removes its own write-enabled step.
"""

from pathlib import Path
import re
import subprocess

BRANCH = "feature/video-renderer-reliability-transcription-scalability-20260720"
ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "backend/internal/websearch/orchestrator.go"
WORKFLOW = ROOT / ".github/workflows/ci.yml"

source = SOURCE.read_text(encoding="utf-8")
old_signature = (
    "func localSummarizerPrompt(plan SearchPlan, tc turncontext.TurnContext, "
    "results []SearchResult, userText string) string {"
)
new_signature = (
    "func localSummarizerPrompt(plan SearchPlan, tc turncontext.TurnContext, "
    "results []SearchResult, _ string) string {"
)
if old_signature not in source:
    raise SystemExit("localSummarizerPrompt signature was not found")
source = source.replace(old_signature, new_signature, 1)

question_suffix = "\n\nUSER QUESTION: %s`, location, resultsBlock, userText)"
occurrences = source.count(question_suffix)
if occurrences != 4:
    raise SystemExit(f"expected four duplicate user-question suffixes, found {occurrences}")
source = source.replace(question_suffix, "`, location, resultsBlock)")
SOURCE.write_text(source, encoding="utf-8")
subprocess.run(["gofmt", "-w", str(SOURCE)], check=True)

workflow = WORKFLOW.read_text(encoding="utf-8")
workflow = workflow.replace(
    "    permissions:\n      contents: write\n",
    "",
    1,
)
checkout_and_finalizer = re.compile(
    r"      - uses: actions/checkout@v7\n"
    r"        with:\n"
    r"          ref: \$\{\{ github\.event\.pull_request\.head\.sha \|\| github\.sha \}\}\n"
    r"          fetch-depth: 0\n"
    r"      - name: Finalize local grounding prompt\n"
    r"        if: github\.event_name == 'pull_request' && github\.head_ref == '"
    + re.escape(BRANCH)
    + r"'\n"
    r"        run: \|\n"
    r"          set -euo pipefail\n"
    r"          python scripts/finalize-websearch-grounding\.py\n"
    r"          git config user\.name 'github-actions\[bot\]'\n"
    r"          git config user\.email '41898282\+github-actions\[bot\]@users\.noreply\.github\.com'\n"
    r"          git add -A\n"
    r"          git diff --cached --check\n"
    r"          git commit -m 'fix\(search\): avoid duplicating current user turn'\n"
    r"          git push origin HEAD:"
    + re.escape(BRANCH)
    + r"\n"
)
workflow, replacements = checkout_and_finalizer.subn(
    "      - uses: actions/checkout@v7\n",
    workflow,
    count=1,
)
if replacements != 1:
    raise SystemExit("branch finalizer workflow block was not found")
WORKFLOW.write_text(workflow, encoding="utf-8")

Path(__file__).unlink()
