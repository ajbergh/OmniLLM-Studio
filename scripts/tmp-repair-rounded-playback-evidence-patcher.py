from pathlib import Path

patcher = Path('scripts/tmp-add-rounded-rectangle-playback-evidence.py')
text = patcher.read_text()
marker = "\nworkflow = '.github/workflows/video-playback-canonical-parity.yml'\n"
if text.count(marker) != 1:
    raise SystemExit(f'workflow patch marker count={text.count(marker)}, want 1')
prefix = text.split(marker, 1)[0]

suffix = r'''
workflow = '.github/workflows/video-playback-canonical-parity.yml'
p = Path(workflow)
text = p.read_text()
old_paths = "      - 'frontend/src/components/video/previewCursorPlayback.ts'\n      - 'frontend/src/components/video/previewCursorPlayback.test.ts'"
new_paths = old_paths + "\n      - 'frontend/src/components/video/previewShapePlayback.ts'\n      - 'frontend/src/components/video/previewShapePlayback.test.ts'\n      - 'frontend/src/components/video/PreviewCanonicalPainters.tsx'\n      - 'frontend/src/components/video/PreviewCanonicalPainters.test.ts'\n      - 'frontend/test/renderContractFrameStateShape.test.ts'"
if text.count(old_paths) != 2:
    raise SystemExit(f"{workflow}: playback path anchor count={text.count(old_paths)}, want 2")
text = text.replace(old_paths, new_paths, 2)
old_tests = '''              src/components/video/previewCursorPlayback.test.ts \\
              src/components/video/previewPlaybackCursorCanonicalization.test.ts \\
              src/components/video/previewTextPlaybackRuntime.test.ts'''
new_tests = '''              src/components/video/previewCursorPlayback.test.ts \\
              src/components/video/previewShapePlayback.test.ts \\
              src/components/video/PreviewCanonicalPainters.test.ts \\
              test/renderContractFrameStateShape.test.ts \\
              src/components/video/previewPlaybackCursorCanonicalization.test.ts \\
              src/components/video/previewTextPlaybackRuntime.test.ts'''
if text.count(old_tests) != 1:
    raise SystemExit(f"{workflow}: focused test anchor count={text.count(old_tests)}, want 1")
text = text.replace(old_tests, new_tests, 1)
old_fixture = '--fixture output/video-playback-canonical/fixture/parity-playback-canonical-v6.json'
if text.count(old_fixture) != 1:
    raise SystemExit(f"{workflow}: fixture anchor count={text.count(old_fixture)}, want 1")
text = text.replace(old_fixture, '--fixture output/video-playback-canonical/fixture/parity-playback-canonical-v7.json', 1)
p.write_text(text)
'''

patcher.write_text(prefix + marker + suffix)
