> **Archived — historical test evidence.** It is retained for transport compatibility context, not as current validation.

# MCP Real-World Test Results

Last updated: 2026-05-09

## Purpose

Validate OmniLLM-Studio's MCP client against a real external MCP server process, not only package-local mocks or protocol fixtures.

## External Server Under Test

- Server: `@modelcontextprotocol/server-filesystem@2025.8.21`
- Transport: stdio
- Launch method: `npx -y @modelcontextprotocol/server-filesystem@2025.8.21 <temp_dir>`
- Scope: a Go test temp directory containing a fixture file

References:

- [MCP stdio transport](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)
- [MCP tools/list and tools/call](https://modelcontextprotocol.io/specification/2025-06-18/server/tools)
- [Filesystem MCP server npm package](https://www.npmjs.com/package/%40modelcontextprotocol/server-filesystem)

## Test Files

- `backend/internal/mcpclient/realworld_test.go`

The tests are opt-in because they require Node/npm, `npx`, and network/package-cache access. They are skipped unless:

```powershell
$env:OMNILLM_RUN_REAL_MCP_TESTS='1'
```

Optional override for systems where `npx` is not on `PATH`:

```powershell
$env:OMNILLM_REAL_MCP_NPX='C:\Program Files\nodejs\npx.cmd'
```

## Tests Added

| Test | What It Verifies |
|---|---|
| `TestRealWorldFilesystemServerClient` | Starts the real filesystem MCP server, runs `initialize`, discovers tools with `tools/list`, calls `read_file`, calls `write_file`, and verifies the written file on disk. |
| `TestRealWorldFilesystemServerManagerAndExecutor` | Persists an MCP server config, starts it through `Manager`, verifies dynamic registry discovery, verifies default `ask` policy blocks execution without an approval handler, switches policy to `allow`, and executes through `tools.Executor`. |

## Commands Run

Default/gated behavior:

```powershell
go test ./internal/mcpclient -run RealWorld -v -count=1
```

Result:

- Passed.
- Both real-world tests skipped as expected because `OMNILLM_RUN_REAL_MCP_TESTS` was not set.

Real external server run:

```powershell
$env:OMNILLM_RUN_REAL_MCP_TESTS='1'
go test ./internal/mcpclient -run RealWorld -v -count=1
```

Result:

- Passed.
- `TestRealWorldFilesystemServerClient`: passed in 5.97s.
- `TestRealWorldFilesystemServerManagerAndExecutor`: passed in 2.01s.
- Package result: `ok github.com/ajbergh/omnillm-studio/internal/mcpclient 8.420s`.

Full regression:

```powershell
go test ./...
```

Result:

- Passed.

## Observations

- The filesystem server logged `Secure MCP Filesystem Server running on stdio`, confirming stdio startup.
- The filesystem server logged that the client does not support MCP Roots and used allowed directories from server args. That is acceptable for the current MVP because the manager launches stdio servers with explicit args.
- `tools/list` discovered expected real server tools including `read_file`, `list_directory`, and `write_file`.
- `tools/call` successfully read a temp fixture file and wrote a new temp file through the real filesystem server.
- Manager integration correctly seeded the discovered MCP `read_file` tool to `ask`; the executor blocked it without an approval handler until the test changed policy to `allow`.
- `npx` emitted a transitive `glob@10.5.0` deprecation warning during the first run. The tests still passed.

## Current Limitations

- Tests are not enabled by default in CI because they depend on external npm package availability.
- The current MCP client does not implement MCP Roots. Filesystem access is provided through command-line allowed directories.
- Interactive approval for `ask` is implemented for Agent Mode. Manual tool execution and the real-world manager test still exercise the no-approval-handler path, so they explicitly switch to `allow`.
