from pathlib import Path

path = Path('backend/internal/video/parity_fixture_playback.go')
text = path.read_text()
replacements = {
    '\t\tDurationMS: 300,\n\t\tTrimOutMS:  300,': '\t\tDurationMS: 400,\n\t\tTrimOutMS:  400,',
    'image := mediaClip("playback-image", "asset-square", 1500, 1200)': 'image := mediaClip("playback-image", "asset-square", 1600, 1100)',
    '\t\t3000,\n\t)\n\tfamilyText := textClip(': '\t\t3100,\n\t)\n\tfamilyText := textClip(',
    'Name: "rounded-rectangle-canonical-playback", FrameIndex: 37, ObserveMS: 250': 'Name: "rounded-rectangle-canonical-playback", FrameIndex: 36, ObserveMS: 250',
    'Name: "ellipse-shape-fallback", FrameIndex: 82, ObserveMS: 250': 'Name: "ellipse-shape-fallback", FrameIndex: 81, ObserveMS: 250',
    'Name: "image-canonical-playback", FrameIndex: 48, ObserveMS: 500': 'Name: "image-canonical-playback", FrameIndex: 51, ObserveMS: 500',
    'Name: "resource-text-canonical-playback", FrameIndex: 93, ObserveMS: 450': 'Name: "resource-text-canonical-playback", FrameIndex: 96, ObserveMS: 450',
}
for old, new in replacements.items():
    count = text.count(old)
    if count != 1:
        raise SystemExit(f'{path}: anchor count={count}, want 1 for {old!r}')
    text = text.replace(old, new, 1)
path.write_text(text)
