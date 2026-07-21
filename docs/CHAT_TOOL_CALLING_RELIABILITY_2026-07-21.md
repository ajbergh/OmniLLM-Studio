# Chat Studio Tool-Calling Reliability

**Branch:** `fix/chat-tool-lifecycle-reliability-20260721`

## Scope

This remediation closes the silent-failure path between provider tool calls, the Go executor, SSE transport, persisted message metadata, and Chat Studio rendering.

## Implemented

- Generic lifecycle SSE events for queued, started, progress, approval, completion, failure, and timeout states
- Generic `tool_result` streaming and durable `tool_results` message metadata for built-in, MCP, plugin, browser, file, memory, job, task, media, and artifact tools
- Safe user-facing tool error codes without exposing raw provider, subprocess, filesystem, or credential details
- Incremental SSE framing that preserves partial network chunks, CRLF, multiline data, and terminal-state correctness
- EOF without `done` or `error` is now an interrupted turn rather than a synthetic success
- Partial assistant output is retained when a stream fails
- Inline Chat Studio approvals resume the original executor/model turn with edited arguments and periodic progress heartbeats
- Policy-denied tools are no longer advertised to models
- Intent-based bounded tool catalogs replace the all-tools payload
- Sparse streamed tool-call indexes and missing provider call IDs are handled deterministically
- Tool-result and loop limits force one final answer-only model call instead of returning an incomplete success
- Compound current-information requests can use composable `web_search` plus calculation, browser, memory, task, or generation tools
- Frontend Gemini tool capability now matches the backend thought-signature implementation

## Compatibility

`browser_tool_results` remains readable for existing conversations. New turns persist the provider-neutral `tool_results` array and continue to include browser-specific metadata where needed for screenshots and navigated URLs.

## Validation

Focused coverage includes backend catalog selection, denied policies, sparse tool-call ordering, safe error mapping, compound search routing, inline approval continuation with edited arguments, and frontend SSE byte-boundary/CRLF/multiline/incomplete-frame behavior. The repository PR quality gate remains authoritative for the full Go, TypeScript, Playwright, race, build, and deployment checks.
