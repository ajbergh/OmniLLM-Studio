# Chat Studio Provider Tool-Calling Conformance

**Branch:** `feat/chat-tool-provider-conformance-observability-20260721`

## Scope

This follow-up to the Chat Studio tool lifecycle remediation hardens provider-facing tool calling and execution safety across OpenAI, Anthropic, Gemini, OpenRouter, Ollama, OpenAI-compatible gateways, and dynamically registered MCP tools.

## Provider conformance

- Missing provider tool-call IDs are replaced with deterministic IDs derived from provider, tool name, arguments, and index.
- Missing tool-call types default to `function`.
- Empty arguments become an empty JSON object.
- Malformed argument payloads are contained in a valid observable wrapper so normal tool validation can report the failure.
- Completed streamed tool calls are normalized before entering the executor.

## Provider request reliability

- Every chat request receives an OmniLLM request correlation ID.
- Upstream request IDs are captured from common OpenAI/Anthropic/OpenRouter-compatible headers.
- Provider HTTP failures are represented by a normalized error that does not expose raw provider bodies to the UI.
- Retryable HTTP statuses are classified explicitly.
- Non-streaming requests may retry once only before a response body is consumed.
- Streaming requests are never automatically replayed after opening because doing so could duplicate text or tool calls.
- Malformed provider SSE payloads and unclean EOF are treated as protocol failures rather than silent success.

## Tool execution safeguards

- Read-only tools may retry one transient execution failure.
- Side-effecting tools are never automatically retried.
- Successful side-effecting calls are cached by invocation scope and stable call ID so repeated provider calls do not execute the side effect twice.
- The replay cache is bounded and is intentionally process-local; it is a duplicate-execution guard, not durable business state.
- Tool results report attempt counts and retry lifecycle events.

## MCP safety

- MCP `readOnlyHint:true` is mapped to a read-only, parallel-eligible local tool definition.
- MCP tools without a valid read-only annotation are conservatively treated as potentially side-effecting.
- Unannotated remote MCP tools therefore cannot become automatically retryable through the legacy local-tool defaults.
- MCP annotations remain advisory; OmniLLM still applies its own permissions and approval policies around the resulting tool definition.

## Mutation metadata audit

Verified side-effect classification for memory writes/deletes, scheduled task creation/updates, asynchronous job cancellation, app/MCP connection changes, and image, music, video, and artifact generation jobs.

## Validation

Focused tests cover provider normalization for OpenAI, Anthropic, Gemini, OpenRouter, Ollama, and generic OpenAI-compatible providers; stable missing IDs; malformed arguments; retryable provider statuses; request ID extraction; safe pre-response HTTP retries; read-only tool retries; side-effect replay protection; and conservative MCP annotation handling.

The repository Quality Gate, Security Scan, and container workflows remain authoritative before merge.
