from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    target = Path(path)
    text = target.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: anchor count={count}, want 1 for {old[:120]!r}")
    target.write_text(text.replace(old, new, 1))


replace_once(
    "frontend/src/components/video/PreviewCanonicalPainters.tsx",
    "import {\n  CURSOR_STATE_CONTRACT_V1,\n  type CanonicalEvaluatedCursorState,\n} from '../../video/renderContractCursor';",
    "import {\n  CURSOR_STATE_CONTRACT_V1,\n  CURSOR_STATE_CONTRACT_V2,\n  type CanonicalEvaluatedCursorState,\n} from '../../video/renderContractCursor';",
)
replace_once(
    "frontend/src/components/video/PreviewCanonicalPainters.tsx",
    "function validateCanonicalCursorState(cursor: CanonicalEvaluatedCursorState): void {\n  if (cursor.contract_version !== CURSOR_STATE_CONTRACT_V1 || cursor.visible !== true) {\n    throw new Error(`canonical preview cursor requires visible ${CURSOR_STATE_CONTRACT_V1}`);\n  }\n}",
    "function validateCanonicalCursorState(cursor: CanonicalEvaluatedCursorState): void {\n  const supportedContract = cursor.contract_version === CURSOR_STATE_CONTRACT_V1\n    || cursor.contract_version === CURSOR_STATE_CONTRACT_V2;\n  if (!supportedContract || cursor.visible !== true) {\n    throw new Error(`canonical preview cursor requires visible ${CURSOR_STATE_CONTRACT_V1} or ${CURSOR_STATE_CONTRACT_V2}`);\n  }\n}",
)
replace_once(
    "frontend/src/components/video/PreviewCanonicalPainters.test.ts",
    "  it('uses exact sampled cursor position, scale, and click state geometry', () => {\n    expect(resolveCanonicalPreviewCursorGeometry(cursorState(), 0.5)).toEqual({\n      left: 60,\n      top: 40,\n      size: 48,\n    });\n  });",
    "  it('uses exact sampled cursor position, scale, and click state geometry for v1 and v2', () => {\n    expect(resolveCanonicalPreviewCursorGeometry(cursorState(), 0.5)).toEqual({\n      left: 60,\n      top: 40,\n      size: 48,\n    });\n    expect(resolveCanonicalPreviewCursorGeometry(cursorState({ contract_version: 'cursor-state-v2' }), 0.5)).toEqual({\n      left: 60,\n      top: 40,\n      size: 48,\n    });\n  });",
)
replace_once(
    "frontend/src/components/video/PreviewCanonicalPainters.test.ts",
    ").toThrow(/cursor-state-v1/);",
    ").toThrow(/cursor-state-v1.*cursor-state-v2/);",
)
