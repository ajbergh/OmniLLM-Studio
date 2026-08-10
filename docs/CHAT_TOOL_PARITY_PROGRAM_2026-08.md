# Chat Studio Tool and Agent Parity Program — August 2026

This program tracks the implementation sequence for bringing OmniLLM-Studio's Chat Studio tool, agent, and extensibility experience to functional parity with mature open-source chat runtimes while preserving OmniLLM-Studio's local-first architecture and creative-studio differentiation.

## Invariants

1. A tool that is **Off** is neither advertised nor executed by Chat Studio or Agent Mode.
2. A tool that is **Ask** remains discoverable but cannot execute without approval.
3. Per-chat and per-turn controls may narrow access but never widen a hard Settings deny.
4. Deterministic routing may select a capability, but it must use the same policy and execution boundary as model-selected tools.
5. Capability availability, policy, provider support, credentials, and runtime health are separate states.
6. Large tool catalogs should be discovered progressively instead of injecting every schema into every turn.

## Phases

### Phase 3 — Unified capability gateway

- Gate deterministic sports, web search, File Library, URL context, image generation, Word generation, and artifact generation through the shared effective tool policy.
- Suppress capability-specific prompt advertising when the corresponding tool is denied or unavailable.
- Keep Ask paths out of deterministic direct execution so they fall through to the approval-aware tool loop.

### Phase 4 — Per-turn controls and tri-state UX

- Add `tool_mode`, `allowed_tools`, and `required_tool` to the Chat send contract.
- Intersect turn restrictions with global effective policy.
- Add a composer tool picker.
- Replace the binary Settings tool toggle with explicit Allow / Ask / Off controls.

### Phase 5 — Reusable agents and Skills

- Add reusable agent profiles that package provider/model, instructions, tool restrictions, and Skills.
- Add Markdown Skills with progressive discovery/read tools instead of injecting all skill bodies into every prompt.

### Phase 6 — OpenAPI and MCP integration parity

- Add OpenAPI-backed tool servers alongside MCP.
- Support operation allowlists, safe HTTP execution, per-tool policies, and readiness diagnostics.
- Improve remote MCP authentication/ownership boundaries where applicable.

### Phase 7 — Sandboxed code execution and programmatic tool orchestration

- Add an external sandbox execution contract, disabled unless configured.
- Route code execution through bounded sessions rather than unrestricted host execution.
- Add a governed programmatic orchestration primitive for loops/batches over authorized registered tools.

### Phase 8 — Scoped permissions and deferred ToolSearch

- Compose policy across system/user/workspace/conversation/turn scopes with deny monotonicity.
- Add ToolSearch/deferred discovery for large MCP/OpenAPI catalogs.
- Persist decision/audit context sufficient to explain why a tool was available or blocked.

## Validation

Each phase should pass:

```bash
cd backend
gofmt -w <changed-go-files>
go vet ./...
go test ./...
go test -race ./...

cd ../frontend
npm run lint
npm run test:unit
npm run build

cd ..
npx playwright test --project=chromium
```

Focused tests should additionally cover policy intersection, approval behavior, disabled-tool discovery suppression, provider compatibility, and regression paths for deterministic shortcuts.
