# OmniLLM-Studio frontend

The frontend is the React 19, TypeScript, Vite, Tailwind, and Zustand client for OmniLLM-Studio. It includes Chat, Image, Music, Video Studio, and Video Edit Studio.

## Development

Use Node.js 24 or newer.

```bash
npm ci
npm run dev
```

The development server proxies API calls to the backend. Set `OMNILLM_API_PROXY_TARGET` when the backend is not running at its default local address.

## Verification

```bash
npm run lint
npm run test:unit
npm run test:video-performance
npm run build
```

`test:video-performance` exercises the large Motion Design fixture (2,000 clips and 16,000 keyframes) and records index, frame-computation, memory, document-size, and patch-size evidence. Browser coverage, including the Motion Design smoke test, runs from the repository root through Playwright.

## Video Edit Studio

Video Edit Studio is one Zustand-backed timeline editor with five modes. Motion Design adds a Design / Animate / Effects / AI / Export workflow, scenes, cameras, 2.5D preview composition, animation blocks, editable curves, cinematic scene effects, and remixable templates while retaining the normal NLE timeline and undo behavior.

The authoritative product references are in [`../docs/VIDEO_STUDIO.md`](../docs/VIDEO_STUDIO.md), [`../docs/VIDEO_TIMELINE_SCHEMA.md`](../docs/VIDEO_TIMELINE_SCHEMA.md), and [`../docs/VIDEO_MOTION_DESIGN_ROADMAP_2026-08.md`](../docs/VIDEO_MOTION_DESIGN_ROADMAP_2026-08.md).
