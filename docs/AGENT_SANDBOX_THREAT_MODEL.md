# Agent Sandbox Threat Model

## Scope

This threat model covers processes or workloads whose behavior may be influenced by an LLM, a user-provided tool/extension, downloaded code/content, or an external MCP/plugin implementation.

Covered execution surfaces include:

- arbitrary code execution;
- terminal/build/test commands;
- restricted Python analysis;
- local plugins;
- stdio MCP servers;
- sandboxed workspace file operations;
- server/Kubernetes sandbox workers;
- durable/background agent tasks that use those execution surfaces.

The headless browser remains a specialized sandbox but shares relevant network/ownership principles.

## Trust boundaries

### Trusted control plane

The following are trusted to enforce application policy:

- authenticated API and invocation scope;
- tool Registry and Executor;
- scoped permission repositories;
- Sandbox Broker;
- workspace registry/mapping;
- credential broker;
- artifact promotion/validation;
- guarded Git/GitHub services;
- operator deployment configuration.

Bugs in these components are in scope as security defects.

### Untrusted or partially trusted inputs

Treat as untrusted:

- model tool arguments;
- generated source/shell code;
- user-provided files that can influence commands;
- sandbox stdout/stderr;
- plugin output;
- MCP output;
- fetched web content;
- package/build scripts;
- repository content, including instructions embedded in source/docs;
- sandbox artifact metadata supplied by a worker;
- remote worker responses until authenticated/validated.

Tool output is data, not higher-authority instruction text.

### Untrusted workloads

The sandbox must assume the workload may intentionally attempt escape, reconnaissance, persistence, credential theft, lateral movement, denial of service, or cross-tenant access.

## Protected assets

The sandbox architecture protects:

- host filesystem outside explicit workspace mounts;
- other user/workspace/conversation/task data;
- backend SQLite database unless explicitly exposed through normal APIs;
- attachment/media storage outside explicit grants;
- provider credentials;
- GitHub credentials;
- `OMNILLM_MASTER_KEY` and seed/key material;
- auth/session tokens;
- browser cookies/profile state;
- SSH/GPG agents and sockets;
- cloud instance/workload metadata credentials;
- host network services;
- other sandbox sessions/workers;
- primary API process availability;
- configured Git repositories not granted to the task;
- plugin directories/configuration not granted to the workload.

## Security goals

### Filesystem confinement

A workload can access only:

- its ephemeral scratch filesystem;
- application-approved workspace mounts in the configured mode;
- runtime files intentionally exposed read-only.

It cannot escape through path traversal, symlinks, junctions/reparse points, hard links, rename races, special files, procfs-like interfaces, mount manipulation, case normalization, or alternate path syntaxes.

### Process confinement

A workload cannot gain additional host privileges by spawning descendants, shells, interpreters, debuggers, or daemons. Descendants inherit restrictions and are terminated when the execution/session is cancelled or destroyed.

### Network confinement

Network access is absent unless policy allows it. When allowed, the workload cannot use alternative addressing/proxy/DNS mechanisms to reach blocked destinations or host/private/metadata networks.

### Credential isolation

No ambient backend secret is present in workload environment, filesystem, inherited handles/descriptors, local sockets, or process metadata. Authenticated operations use host-side brokers with task-scoped authority.

### Resource containment

A workload cannot indefinitely consume host CPU, memory, process slots, storage, file count, or output buffers beyond configured policy. A single workload must not trivially starve the primary application or other tenants.

### Ownership isolation

Sandbox/session/artifact/workspace IDs are references, not authorization. Every operation revalidates authenticated owner scope and relevant workspace/task relationship.

### Auditability and user control

Sensitive actions remain represented through existing tool policy/approval semantics. Workspace mutations are journaled. Users can inspect current grants and resulting changes.

## Threat actors

### Prompt-injected model execution

A benign model is induced by repository/web/tool content to run commands, exfiltrate files, or weaken safeguards.

Mitigation: model intent is never the security boundary; sandbox and tool policy enforce hard limits.

### Malicious user in multi-user deployment

An authenticated user intentionally crafts tool calls/content to escape into another tenant or host resources.

Mitigation: ownership binding, worker separation, mount isolation, no ambient credentials, quotas.

### Malicious plugin or MCP server

Installed local extension code attempts arbitrary host access after launch.

Mitigation: run extensions under the same sandbox policy; manifest capability declarations are requests, not grants.

### Compromised dependency/build script

A package install/build/test command executes hostile post-install or build logic.

Mitigation: terminal is sandbox-only; network/workspace/secret constraints remain enforced for descendants.

### Compromised sandbox worker

A remote worker sends forged artifact metadata or attempts to exceed assigned task scope.

Mitigation: authenticated channel, application-owned owner/policy state, artifact hash/size validation, worker has no broader tenant credentials.

## Attack classes and required tests

## 1. Filesystem escape

Attempt:

- `../` and mixed-separator traversal;
- Unix and Windows absolute paths;
- symlink parent and final-component attacks;
- Windows junction/reparse points;
- hard links to outside targets;
- rename/move outside root;
- case-insensitive aliasing;
- UNC/device/alternate-data-stream paths where applicable;
- `/proc`, `/sys`, device files and runtime sockets;
- mount namespace manipulation;
- delete/recreate tricks against `read_write_no_delete`;
- TOCTOU path replacement during write/patch.

Pass condition: outside resources remain unreadable/unwritable and no physical host path is disclosed unnecessarily.

## 2. Process escape and persistence

Attempt:

- fork/process bomb;
- nested shell/interpreter;
- detached/background process;
- daemonization;
- orphan child after parent timeout;
- debugger/ptrace-style access;
- inherited descriptors/handles;
- creating scheduled/startup persistence;
- signalling unrelated host processes.

Pass condition: process count is bounded, unrelated host processes are inaccessible, and cancel/destroy removes descendants.

## 3. Network bypass

Attempt:

- localhost/loopback;
- RFC1918/private ranges;
- link-local;
- cloud metadata endpoints;
- IPv6 equivalents;
- DNS rebinding;
- allowed hostname resolving to blocked address;
- direct IP when only hostname is allowed;
- HTTP redirect to blocked destination;
- proxy environment variables;
- custom DNS/resolver tricks;
- Unix/named sockets;
- host gateway/container bridge addresses.

Pass condition: only policy-approved destination/port/protocol combinations are reachable.

## 4. Credential theft

Attempt to obtain:

- all environment variables from backend/worker parent;
- provider API keys;
- master key/seed files;
- database/session tokens;
- GitHub token/app credentials;
- SSH/GPG agent sockets;
- browser profile/cookies;
- Kubernetes service-account credentials;
- cloud metadata credentials;
- shell history/user profile secrets.

Pass condition: none are ambient in sandbox; brokered operations reveal only bounded result data.

## 5. Resource denial of service

Attempt:

- infinite CPU loop;
- allocator/memory bomb;
- recursive child spawning;
- disk fill;
- sparse/truncate expansion;
- millions of small files;
- stdout/stderr flood;
- large artifact creation;
- long-running sleeping processes designed to consume concurrency slots.

Pass condition: configured limits terminate or reject the workload and the main application remains responsive.

## 6. Cross-tenant/session access

Attempt to use a known/guessed ID from another:

- user;
- workspace;
- conversation;
- message;
- task;
- agent run;
- sandbox;
- artifact.

Pass condition: every mismatch fails authorization even when the object ID exists.

## 7. Artifact attacks

Attempt:

- worker-provided URL pointing to internal/metadata/private host;
- MIME spoofing;
- oversized artifact;
- path traversal artifact name;
- symlink artifact;
- artifact changed after reported hash;
- cross-user artifact ID reuse.

Pass condition: promotion uses application-owned IDs, bounded transfer, content/path validation, hash verification, and owner checks.

## 8. Workspace journal/revert attacks

Attempt:

- mutate between hash and write;
- exploit journal to revert someone else's changes;
- huge file to force unbounded before-content persistence;
- binary/symlink edge cases;
- delete without delete permission;
- replay stale change/revert request.

Pass condition: writes/reverts use ownership and state preconditions; journal remains bounded and accurate.

## 9. Git boundary bypass

Attempt:

- use terminal to inject hosted credentials;
- modify `.git` to trick reviewed state;
- stage/commit/push outside guarded Git flow when policy intends reviewed publication;
- symlink worktree path outside configured repo;
- race a reviewed diff before staging.

Pass condition: existing guarded Git state-binding remains authoritative for reviewed stage/commit/publication flows; raw hosted credentials are not ambient in terminal.

## 10. Prompt/tool-result trust attacks

Place instructions in:

- source files;
- build output;
- sandbox stdout;
- MCP response;
- plugin response;
- downloaded package metadata;
- web pages.

Pass condition: these inputs do not alter sandbox/tool policy and do not gain higher instruction authority because they were emitted by a tool.

## 11. Runtime capability downgrade

Attempt to configure/use a runtime that cannot enforce required controls.

Pass condition: runtime reports enforcement capabilities; deployments configured to require a control fail closed rather than silently using unrestricted host execution.

## Abuse-resistant defaults

Recommended defaults:

```text
workspace: read_only unless task explicitly needs writes
scratch: read_write ephemeral
workspace delete: denied/Ask
network: none
ambient environment: none
credentials: none
terminal/code: Ask/high-risk
plugins/MCP local process: sandbox required in hardened/multi-user profiles
artifact size/count: bounded
session TTL: bounded
```

Desktop developer compatibility modes may be less strict only when explicit and visibly reported. Multi-user/server profiles should not silently downgrade.

## Security review gates

A phase is not complete until:

- threat-relevant unit tests exist;
- applicable platform-native containment tests run on that platform;
- negative/escape cases are included, not only happy paths;
- runtime capability reporting is verified;
- docs state any unsupported enforcement control explicitly;
- security-sensitive CI is green or a repository-approved temporary exception is documented.

## Out of scope / non-goals

- Proving mathematical non-interference against kernel vulnerabilities.
- Treating containers alone as a complete security proof.
- Treating plugin signing/provenance as runtime isolation.
- Allowing user approval to bypass hard operator/deployment restrictions.
- Replacing existing SSRF-safe HTTP transports or guarded Git state-binding with generic shell access.

## Incident posture

Sandbox failures must avoid exposing raw host paths, secrets, or unrestricted subprocess diagnostics to clients. Security-relevant failures should produce structured internal audit events suitable for diagnosis. Ambiguous worker/session state after a transport failure should require fresh status/ownership inspection rather than blind retry of side effects.
