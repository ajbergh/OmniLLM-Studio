#!/usr/bin/env python3
"""Apply the final local-grounding fix, then remove this one-shot helper.

The CI job commits only the source correction and this deletion. The workflow is
restored separately through the repository API because GitHub's workflow token
cannot push a commit that modifies workflow files.
"""

from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "backend/internal/websearch/orchestrator.go"

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

Path(__file__).unlink()
