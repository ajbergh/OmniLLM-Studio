# Video parity report structural-region input

`video-parity-report` can optionally attach exact structural regions to decoded frame pairs through `--regions <path>`.

The input is a versioned JSON manifest keyed by canonical integer `frame_index`. Each frame may define named rectangular regions using `min_x`, `min_y`, `max_x`, and `max_y` pixel bounds. Invalid versions, unknown fields, duplicate frame indices, duplicate region names, empty names, negative coordinates, empty/inverted rectangles, multiple JSON values, rectangles extending outside either decoded frame, and manifest frames with configured regions but no matching preview/rendered PNG pair all fail closed before comparison.

Omitting `--regions` preserves the existing parity-report behavior and attaches no structural regions.

This input boundary does **not** by itself define the production parity policy. `ParityRegion` currently means exact decoded RGB equality. Codec-affected image areas therefore must not be labeled exact merely to satisfy a structural gate. Production sign-off should use canonical FrameState/composition diagnostics for zero-tolerance identity/geometry and reserve exact decoded regions for genuinely codec-independent structure; tolerance-aware decoded-region policy remains separate follow-on work.

See `testdata/regions-v1.json` for the schema shape.
