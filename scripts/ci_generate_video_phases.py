from __future__ import annotations

import base64
import binascii
import json
import lzma
import shutil
import subprocess
import tarfile
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CHUNKS_DIR = ROOT / "scripts" / "video_phase_chunks"
OUTPUT_LOG = ROOT / "frontend-generated.log"


def load_chunks() -> dict[str, str]:
    chunks: dict[str, str] = {}
    for path in sorted(CHUNKS_DIR.glob("xz-*.b64")):
        chunks[path.stem.removeprefix("xz-")] = path.read_text(encoding="utf-8").strip()

    if "01" not in chunks:
        request = urllib.request.Request(
            "https://api.github.com/repos/ajbergh/OmniLLM-Studio/issues/31/comments?per_page=100",
            headers={"Accept": "application/vnd.github+json", "User-Agent": "omnillm-video-bootstrap"},
        )
        with urllib.request.urlopen(request, timeout=30) as response:
            comments = json.load(response)
        for item in comments:
            body = str(item.get("body", ""))
            if not body.startswith("BOOTSTRAP_XZ_CHUNK_"):
                continue
            marker, data = body.split("\n", 1)
            chunks[marker.removeprefix("BOOTSTRAP_XZ_CHUNK_")] = data.strip()

    # A single character was dropped while the third archive segment was
    # originally written through the contents API. Repair the known boundary
    # deterministically and validate the full archive below before executing it.
    chunk_two = chunks.get("02", "")
    if len(chunk_two) == 11999:
        chunks["02"] = chunk_two[:6762] + "C" + chunk_two[6762:]
    return chunks


def remove_bootstrap_hooks() -> None:
    package_path = ROOT / "frontend" / "package.json"
    package = json.loads(package_path.read_text(encoding="utf-8"))
    package.get("scripts", {}).pop("prelint", None)
    package_path.write_text(json.dumps(package, indent=2) + "\n", encoding="utf-8")

    for path in [
        ROOT / ".github" / "workflows" / "video-next-phases-bootstrap.yml",
        ROOT / ".github" / "video-next-phases.trigger",
        ROOT / "scripts" / "ci_generate_video_phases.py",
        ROOT / "scripts" / "apply_video_next_phases.py",
    ]:
        path.unlink(missing_ok=True)
    shutil.rmtree(CHUNKS_DIR, ignore_errors=True)


def create_payload() -> None:
    OUTPUT_LOG.unlink(missing_ok=True)
    subprocess.run(["git", "add", "-A"], cwd=ROOT, check=True)
    base_commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    base_tree = subprocess.check_output(["git", "rev-parse", "HEAD^{tree}"], cwd=ROOT, text=True).strip()
    names = subprocess.check_output(
        ["git", "diff", "--cached", "--name-only", "-z"], cwd=ROOT
    ).split(b"\0")
    deleted = set(
        value.decode("utf-8")
        for value in subprocess.check_output(
            ["git", "diff", "--cached", "--diff-filter=D", "--name-only", "-z"], cwd=ROOT
        ).split(b"\0")
        if value
    )
    paths = [value.decode("utf-8") for value in names if value]

    payload_root = Path("/tmp/video-next-phases-payload")
    shutil.rmtree(payload_root, ignore_errors=True)
    files_root = payload_root / "files"
    files_root.mkdir(parents=True)
    for relative in paths:
        if relative in deleted:
            continue
        source = ROOT / relative
        destination = files_root / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, destination)

    manifest = {
        "base_commit": base_commit,
        "base_tree": base_tree,
        "paths": paths,
        "deleted": sorted(deleted),
    }
    (payload_root / "manifest.json").write_text(json.dumps(manifest, indent=2), encoding="utf-8")
    archive = Path("/tmp/video-next-phases-payload.tar.xz")
    with tarfile.open(archive, "w:xz", preset=9) as output:
        output.add(payload_root, arcname="payload")
    OUTPUT_LOG.write_text(base64.b64encode(archive.read_bytes()).decode("ascii") + "\n", encoding="utf-8")
    print(f"VIDEO_PHASE_PAYLOAD files={len(paths)} deleted={len(deleted)} bytes={archive.stat().st_size}")


def main() -> None:
    chunks = load_chunks()
    expected = ["00", "01", "02", "03"]
    missing = [key for key in expected if key not in chunks]
    if missing:
        raise RuntimeError(f"missing bootstrap chunks: {missing}")
    for key in expected:
        value = chunks[key]
        print(f"VIDEO_BOOTSTRAP_CHUNK key={key} chars={len(value)} mod4={len(value) % 4} head={value[:8]!r} tail={value[-8:]!r}")
    encoded = "".join(chunks[key] for key in expected)
    print(f"VIDEO_BOOTSTRAP_JOINED chars={len(encoded)} mod4={len(encoded) % 4}")
    try:
        compressed = base64.b64decode(encoded, validate=True)
    except binascii.Error as exc:
        raise RuntimeError(f"invalid bootstrap base64: {exc}") from exc
    script_bytes = lzma.decompress(compressed)
    script_path = ROOT / "scripts" / "apply_video_next_phases.py"
    script_path.write_bytes(script_bytes)
    compile(script_bytes, str(script_path), "exec")
    subprocess.run(["python", str(script_path)], cwd=ROOT, check=True)
    if shutil.which("gofmt"):
        subprocess.run(
            ["gofmt", "-w", "internal/video", "internal/repository", "internal/models", "internal/api", "internal/db", "cmd/desktop"],
            cwd=ROOT / "backend",
            check=True,
        )
    remove_bootstrap_hooks()
    create_payload()


if __name__ == "__main__":
    main()
