from pathlib import Path

path = Path('backend/internal/video/parity_fixture_playback.go')
text = path.read_text()
replacements = {
    'Name: "rounded-rectangle-canonical-playback", FrameIndex: 37, ObserveMS: 250': 'Name: "rounded-rectangle-canonical-playback", FrameIndex: 36, ObserveMS: 200',
    'Name: "ellipse-shape-fallback", FrameIndex: 82, ObserveMS: 250': 'Name: "ellipse-shape-fallback", FrameIndex: 81, ObserveMS: 200',
}
for old, new in replacements.items():
    count = text.count(old)
    if count != 1:
        raise SystemExit(f'{path}: anchor count={count}, want 1 for {old!r}')
    text = text.replace(old, new, 1)
path.write_text(text)
