# Video render parity fixtures

`parity-torture-v1` is generated from `video.ParityTortureFixture` so IDs,
timeline values, frame-boundary rules, and seeded samples stay deterministic.
Binary media is generated locally instead of committed because codec output is
platform-dependent; comparisons are performed on decoded frames and PCM.

From `backend/`:

```powershell
go run ./cmd/video-parity-fixture --output-dir ../video-renderer/test/fixtures/generated --generate-media
```

The generated bundle contains the validated timeline, source-asset recipes,
and named frame indices. The fixed seed is `20260817` and the default canvas is
640×360 at 30 fps.
