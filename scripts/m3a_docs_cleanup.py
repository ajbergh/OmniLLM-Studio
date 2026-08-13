from pathlib import Path


def replace_all(path: str, replacements: list[tuple[str, str]]) -> None:
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    for old, new in replacements:
        if old not in text:
            raise SystemExit(f"missing expected text in {path}: {old!r}")
        text = text.replace(old, new)
    p.write_text(text, encoding="utf-8")

replace_all(
    "docs/GITHUB_PULL_REQUEST_MERGE_DESIGN_2026-08.md",
    [
        ("M3 should add a separate process gate and per-remote permission", "M3B should add a separate process gate and per-remote permission"),
        ("a previous M1/M2 result", "a previous M1/M2/M3A result"),
        ("Limit M3 model inputs to:", "Limit M3B model inputs to:"),
        ("Immediately before any merge request, M3 must:", "Immediately before any merge request, M3B must:"),
        ("fresh M2/current-state preflight", "fresh M3A preflight"),
        ("M3 must fail closed.", "M3B must fail closed."),
        ("## Validation plan for M3", "## Validation plan for M3B"),
        ("Before any M3 merge:", "Before any M3B merge:"),
        ("M1–M3 do not combine or implicitly authorize:", "M1–M3B do not combine or implicitly authorize:"),
    ],
)

replace_all(
    "docs/GITHUB_PULL_REQUEST_MERGE_REQUIREMENTS.md",
    [
        ("Before merging M2 or any future M3 work,", "Before merging M3A or any future M3B work,"),
    ],
)

replace_all(
    "docs/CHAT_TOOL_PARITY_PROGRAM_2026-08.md",
    [
        ("Required M3 boundary:", "Required M3B boundary:"),
        ("stale M1/M2 reads", "stale M1/M2/M3A reads"),
    ],
)
