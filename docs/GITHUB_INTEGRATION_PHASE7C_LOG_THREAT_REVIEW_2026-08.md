# GitHub Integration Phase G7C — CI Log Threat Review — August 2026

> **Decision:** DO NOT expose raw GitHub Actions job logs in the current G7 read surface.
>
> G7A check annotations and G7B workflow/job/step metadata provide the initial CI-diagnosis parity layer without requiring OmniLLM to ingest arbitrary workflow log streams. A future bounded textual excerpt tool is conditional on the safeguards and authorization work below; it is not implicitly authorized by `allow_pull_request_read`.

## Context

G7A and G7B intentionally keep provider object selection outside model control:

- the model selects only a configured remote ID and pull-request number;
- the backend fetches the PR fresh;
- the exact PR head is validated;
- check/workflow/job IDs are derived server-side;
- returned provider text/metadata is bounded and treated as untrusted reference data.

Raw CI logs are materially different from check annotations or status metadata. They are arbitrary process output produced by repository code and third-party actions, can be very large, and may contain credentials or other sensitive values despite provider masking.

## Current client boundary that must not be weakened implicitly

The shared GitHub API client currently provides two useful protections:

- API responses are bounded to 1 MiB (`maxGitHubAPIResponseBytes`);
- HTTP redirects are disabled by `newGitHubAPIClient`.

`doGitHubJSON` accepts only repository-scoped `/repos/...` endpoints, targets the fixed `https://api.github.com` API host, and returns sanitized status errors rather than provider response bodies.

A log-download implementation must not disable those protections globally or make the model able to choose a redirect target, signed URL, provider object ID, or arbitrary endpoint.

## Threats introduced by raw job logs

### 1. Secret disclosure

Workflow logs can contain:

- credentials printed by repository scripts;
- third-party action output;
- temporary signed URLs;
- environment or configuration values;
- stack traces containing request data;
- intentionally adversarial strings from pull-request code.

GitHub masking is helpful provider hygiene but is not a sufficient OmniLLM authorization or redaction boundary. Repository code can transform a secret before printing it or expose sensitive non-secret data that the provider does not know to mask.

### 2. Redirect / egress boundary expansion

GitHub Actions log-download endpoints can involve download/redirect behavior that differs from the ordinary JSON REST calls used by G7A/G7B.

The existing GitHub HTTP client intentionally rejects redirects. A future log downloader must therefore use a separate, narrowly scoped fetch path with an explicit destination policy rather than changing `newGitHubAPIClient` to follow redirects.

Required properties:

- initial request remains fixed to `api.github.com` and a backend-derived job/run ID;
- redirect count is strictly bounded;
- redirect scheme must be HTTPS;
- redirect host must match an explicit GitHub-owned download-host allowlist established from current provider documentation and tests;
- credentials/tokens must never be forwarded to the redirected download host unless the provider contract explicitly requires and safely scopes them;
- DNS/IP validation must retain the repository's remote-safe dialing protections;
- redirects to loopback, link-local, private, metadata, or user-controlled hosts fail closed.

### 3. Prompt injection / control confusion

Logs are untrusted repository/process output. They can contain instructions crafted to influence an agent.

Any future log excerpt must be labeled and typed as untrusted diagnostic evidence. Log text must never:

- authorize another tool call;
- change repository/binding selection;
- supply a trusted commit/run/job ID;
- satisfy guarded merge/review evidence requirements;
- override operator approval or side-effect policy.

### 4. Unbounded content and denial of service

Raw logs may be many megabytes and may contain extremely long lines or binary/control content.

A future implementation needs both transport and result bounds before content is exposed to the model.

Minimum proposed bounds:

- fetch at most one backend-derived failed job per call;
- hard transport cap before decompression/decoding: 2 MiB;
- hard decoded cap: 1 MiB;
- expose at most 32 KiB of selected diagnostic text;
- at most 400 lines;
- at most 4 KiB per line;
- strip NUL and non-text control characters;
- explicit `truncated` flags for transport, decoded, line-count, and result truncation.

These numbers are proposed guardrails, not an implemented contract.

### 5. Archive/decompression hazards

If the provider returns an archive, OmniLLM must not extract arbitrary paths to disk.

Any future archive handling must:

- stream in memory or to an isolated runtime-owned temporary file;
- cap compressed and uncompressed bytes independently;
- reject nested archives;
- reject path traversal and absolute member names;
- cap archive member count;
- never write archive paths into the repository/workspace;
- delete runtime-owned temporary material on all completion/error paths.

Prefer a provider format that avoids archive extraction entirely if available.

### 6. Authorization broadening

G7A/G7B fit the existing explicit PR/CI read policy because they return bounded check/status metadata.

Raw job output is a broader data class. It must not silently inherit `allow_pull_request_read` simply because the object is associated with a PR.

A future implementation should add a separate fail-closed authorization control, for example:

```text
OMNILLM_GITHUB_WORKFLOW_LOG_READ_ENABLED=true
allow_workflow_log_read=true
```

Exact naming is proposed, not implemented. Repository bindings must derive this authority explicitly just as G1-G6 bindings derive other permissions; absence means denied.

GitHub App/provider permissions must also be documented separately. OmniLLM authorization cannot grant a provider permission the connected installation/token does not have.

## Redaction requirements before any G7C code

A safe first implementation cannot rely on a single generic secret regex.

At minimum, redaction needs layered handling for:

- exact known credential values already held by the request-scoped credential broker/client, without exposing those values to the model;
- common token/key formats with high-confidence patterns;
- Authorization/Cookie/private-key style headers and blocks;
- URL query parameters commonly used for signed credentials;
- configurable operator redaction patterns for organization-specific secrets.

Redaction must happen before the bounded excerpt enters any model-facing structure, audit text, error message, or diagnostic artifact.

The redactor itself needs adversarial tests for encoded/split/long-line inputs and must fail closed if a required redaction stage errors.

## Selection model if G7C is later approved

The model should still provide only:

```text
remote
number
```

The backend should:

1. fetch the PR fresh;
2. validate its exact head;
3. derive failing workflow/job metadata using the G7B path;
4. deterministically select a bounded failed job, or return a bounded list of diagnostic candidates that can be selected by an opaque OmniLLM-issued reference scoped to that exact PR head;
5. fetch through the dedicated safe download path;
6. redact and bound content;
7. return untrusted diagnostic text with exact-head identity and truncation metadata.

The model must not submit a GitHub run ID, job ID, API URL, signed URL, or arbitrary commit SHA.

## Decision and exit from G7C review

**Current recommendation: defer raw textual log access.**

G7A check annotations plus G7B workflow/job/step status metadata should land and be evaluated first. They provide useful failure localization with a much smaller content and credential surface.

G7C implementation becomes justified only if real use demonstrates a material diagnosis gap that annotations + step metadata cannot cover and the following prerequisites are ready:

- independent workflow-log authorization gate and per-remote/binding policy;
- provider permission documentation;
- dedicated safe redirect/download transport;
- bounded streaming/decompression strategy;
- pre-model redaction engine with adversarial tests;
- untrusted-content typing and no authorization-by-log invariant;
- exact-head/backend-derived job binding tests;
- exact final-head Security/Quality/container validation.

Until then, **no raw Actions log/archive endpoint should be exposed as a model tool** and the shared GitHub API client's redirect prohibition must remain unchanged.

## Next GitHub integration work

After G7B, safer parity work should prioritize bounded collaboration metadata/actions with independent authorization families rather than weakening the CI content boundary. Candidate areas include issues/discussions read surfaces, remote branch lifecycle, and release/tag workflows, each with its own operator policy and threat model.
