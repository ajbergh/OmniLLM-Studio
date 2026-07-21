#!/usr/bin/env python3
"""Remove duplicate current-user text from local web-grounding prompts.

The local summarizer already receives the current user turn as a chat message.
Embedding the same question in its system prompt duplicates the turn and can
change model weighting. This branch-only finalizer patches the large source
file, restores read-only CI, removes itself, and commits once.
"""

from pathlib import Path

BRANCH = "feature/video-renderer-reliability-transcription-scalability-20260720"
ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "backend/internal/websearch/orchestrator.go"
WORKFLOW = ROOT / ".github/workflows/ci.yml"

text = SOURCE.read_text(encoding="utf-8")
text = text.replace(
    "func localSummarizerPrompt(plan SearchPlan, tc turncontext.TurnContext, results []SearchResult, userText string) string {",
    "func localSummarizerPrompt(plan SearchPlan, tc turncontext.TurnContext, results []SearchResult, _ string) string {",
    1,
)
replacements = [
    ("WEB EVIDENCE:\n%s\n\nUSER QUESTION: %s`, location, resultsBlock, userText)", "WEB EVIDENCE:\n%s`, location, resultsBlock)"),
]
for old, new in replacements:
    count = text.count(old)
    if count != 4:
        raise SystemExit(f"expected four local prompt question copies, found {count}")
    text = text.replace(old, new)
SOURCE.write_text(text, encoding="utf-8")

workflow = WORKFLOW.read_text(encoding="utf-8")
workflow = workflow.replace("    permissions:\n      contents: write\n", "", 1)
workflow = workflow.replace(
    "      - uses: actions/checkout@v7\n        with:\n          ref: ${{ github.event.pull_request.head.sha || github.sha }}\n          fetch-depth: 0\n"
    "      - name: Finalize local grounding prompt\n"
    "        if: github.event_name == 'pull_request' && github.head_ref == '" + BRANCH + "'\n"
    "        run: |\n"
    "          set -euo pipefail\n"
    "          python scripts/finalize-websearch-grounding.py\n"
    "          git config user.name 'github-actions[bot]'\n"
    "          git config user.email '41898282+github-actions[bot]@users.noreply.github.com'\n"
    "          git add -A\n"
    "          git diff --cached --check\n"
    "          git commit -m 'fix(search): avoid duplicating current user turn'\n"
    "          git push origin HEAD:" + BRANCH + "\n",
    "      - uses: actions/checkout@v7\n",
    1,
)
WORKFLOW.write_text(workflow, encoding="utf-8")
Path(__file__).unlink()
