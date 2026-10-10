# OmniLLM-Studio 0.3 release acceptance

This is a **release gate**, not a declaration that 0.3 is ready. Do not create
`v0.3.0-rc1` until all checks and the end-to-end workflow below have passed on
the exact release commit.

## Reproducible sources and artifact integrity

- All changes enter `main` through reviewed, validated pull requests.
- `backend/go.mod` and release build tools agree on Go 1.26, Node 24, and
  the Wails CLI/library at v2.16.0. Reconcile any future change together.
- Run the repo Quality Gate, Security Scan, and applicable sandbox, browser,
  and renderer-parity workflows against the final integration revision.
- The release workflow builds Windows/amd64, macOS/arm64, and Linux/amd64.
  Passing a build job alone is not proof of install/runtime support.
- Before publishing, `scripts/stage-release-assets.sh` requires all three
  nonempty outputs, gives each a unique platform filename, writes SHA256SUMS.txt,
  and verifies that manifest. The independent CI fixture exercises corruption
  detection and missing-platform rejection.
- A tag containing a prerelease suffix, such as `v0.3.0-rc1`, must be
  published as a GitHub prerelease. Stable tags are not prereleases.

After downloading release artifacts, verify their integrity from the same
directory:

```bash
sha256sum --check SHA256SUMS.txt
```

Checksums guard accidental damage and substitution relative to the published
manifest; they are not a substitute for code signing, artifact provenance or a
trusted release channel.

## User journey acceptance (must be executed, not inferred)

1. Clean install, initialize owner and verify that no credentials are leaked.
2. Configure a local Ollama model; confirm an offline request never reaches a
   cloud provider. Separately verify explicit cloud-consent behavior.
3. Research with a private file and a public source; ensure citations, retrieval
   status, and failure warnings survive a follow-up tool call and saved history.
4. Generate/edit an image with model-supported controls, then retain its source,
   version and project relationship.
5. Import/generate audio, assemble an editable video timeline and verify media
   references and preview/export fidelity on supported fixtures.
6. Render/export and inspect the resulting playable file, including audio.
7. Restart the app, reopen the saved project and verify asset lineage and
   intended state (undo/history and saved references).
8. Exercise cancellation, invalid keys, unavailable model, missing media,
   provider timeout, disk exhaustion, interrupted sandbox worker, and recovery
   from an interrupted render/task.
9. Validate Windows and macOS installation, clean state and upgrade from 0.2;
   exercise documented rollback. Validate Linux runtime/container deployment
   separately rather than assuming portable binaries are sufficient.

## Publication prerequisites

- Enforce required checks and PR-based merges on `main` (issue #322).
- Close handler integration gaps tracked in #324, including retrieval + tool
  composition and safe stream failures.
- Validate sandbox task startup/shutdown/restart behavior and never advertise
  controls that the runtime does not enforce.
- Record platform test results and known limitations in release notes.
- Supply signing/notarization only where provisioned and tested; otherwise
  accurately label binaries as unsigned and document platform warnings.
- Publish installable artifacts and manifest, then verify downloads and
  `sha256sum --check` from the published release itself.

**Do not tag a release solely because the CI matrix is green.** Release
readiness additionally requires the explicit acceptance evidence above.
