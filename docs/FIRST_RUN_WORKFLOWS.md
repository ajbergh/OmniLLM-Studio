# First-run workflows (0.3)

OmniLLM-Studio is **local-first**, not inherently offline. Conversation history, project state and configured indexes are stored locally by default. A chosen cloud provider receives content supplied in that model request, potentially including selected excerpts of local documents. Web search, external MCP tools, and cloud image/music/video services have their own data and network implications. Do not assume a model or tool is local solely because the application runs on your computer.

These are **recommended setup paths**, not preconfigured one-click presets. The guided in-product profile selector and project-wide asset handoff acceptance remain tracked in [#342](https://github.com/ajbergh/OmniLLM-Studio/issues/342).

## Local-only workstation

1. Run the backend and frontend following the [README](../README.md#get-running).
2. Install and start your own local Ollama runtime and select a discovered local model under **Settings → Providers**.
3. Avoid configuring cloud credentials and do not enable web search, remote MCP tools, external plugins, or cloud generation in sessions intended to remain offline.
4. Verify that the workflow works after network access is intentionally disconnected; absence of cloud credentials alone does not prove every optional tool is offline.

## Hybrid research

1. Configure a supported local and/or cloud chat provider under **Settings → Providers**.
2. Add files to the File Library and use retrieval with citations; inspect which excerpts are included in provider prompts.
3. Make a deliberate provider selection per workflow and validate credentials before relying on external access.
4. Treat internet pages and retrieved text as untrusted input, not instructions for tools.

## Creator workstation

1. Set up the image, music, and/or video providers you intend to use.
2. Confirm model availability and any external usage charges before submitting a generation request.
3. Preserve input assets, variations and output files alongside the project; test that saved video timelines reopen and exported media can be played.
4. Cloud media generation and transcription may transmit prompts, reference imagery, audio or video to the selected provider.

## Developer / agent workstation

1. Start with local or explicitly configured provider credentials.
2. Review permissions for MCP servers, plugins, browser use and tool execution before enabling them.
3. Keep the sandbox no-network unless the selected runtime proves an enforceable network policy; never infer permissions from UI feature availability.
4. Test cancellation, failure and review workflows using non-production data.

## Team / self-hosted deployment

1. Review the [technical reference](TECHNICAL_REFERENCE.md) and [Helm guide](how_to_helm.md).
2. Configure authenticated access, backup procedures, TLS termination and project ownership before admitting other users.
3. Keep arbitrary sandbox execution isolated from the primary API, and validate deployment network/security controls.
4. Confirm the documented single-replica storage constraints before scaling.

## Checks before relying on a workflow

- Which provider or runtime processes each request?
- Can a cloud request include local document or media content?
- Are costs, model failures and unavailable providers reported clearly?
- Does the project reopen after application restart, with no missing assets?
- Do error, cancellation and recovery paths avoid re-executing external side effects?

See the [0.3 release acceptance checklist](RELEASE_0_3_ACCEPTANCE.md) for the full cross-platform release gate. Successful unit tests do not establish completion of those end-to-end checks.
