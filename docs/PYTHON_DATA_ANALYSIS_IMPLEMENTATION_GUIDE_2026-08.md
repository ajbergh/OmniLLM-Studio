# Python Data Analysis Runtime — Implementation Guide

> **Status:** PROPOSED
>
> **Date:** August 2026
>
> **Primary target:** Chat Studio
>
> **Architecture intent:** add a self-contained, reproducible Python runtime and rich data-analysis capability without bypassing the existing Sandbox Broker, tool policy, approval, workspace-grant, artifact, or platform-native confinement boundaries.

## Executive summary

OmniLLM-Studio already has two Python-capable Chat Studio paths:

- `code_execute`, which runs Python, JavaScript, or shell through the application-owned Sandbox Broker; and
- `python_analysis`, which runs a deliberately restricted Python subset through that same Broker.

The proposed work should **not** introduce a third, parallel execution architecture. Instead, it should complete runtime packaging for the existing sandbox platform and then expose a richer, curated analysis environment for Chat Studio.

The recommended product outcome is:

> **Self-contained sandboxed Python + Data Analysis for Chat Studio**

The recommended runtime approach is to use a library such as `github.com/kluctl/go-embed-python` as an **interpreter distribution/extraction mechanism only**. OmniLLM-Studio must continue to create and constrain Python processes through its own sandbox runtimes. In particular, the upstream convenience command helper must not become the Omni execution path because it inherits the host process environment; Omni's sandbox design intentionally removes ambient backend secrets and applies platform-native filesystem, process, network, lifecycle, and resource controls.

The target architecture is:

```text
Chat Studio / Agent tool call
          |
          v
Tool Registry + Executor policy
          |
          v
Sandbox Broker
  - ownership/session validation
  - workspace grants
  - TTL
  - resource admission
  - network/credential policy
          |
          v
Python Runtime Provider
  - embedded desktop runtime
  - preinstalled worker/container runtime
  - exact version/package manifest
          |
          v
Platform sandbox runtime
  - Linux Bubblewrap + cgroup v2 where available
  - Windows AppContainer + Job Objects
  - macOS Seatbelt
          |
          v
Python child process
  - sanitized environment
  - no network by default
  - bounded workspace input
  - bounded artifact output
```

This design preserves the current local-first architecture and creates a path toward a ChatGPT-style code-interpreter/data-analysis experience without weakening the sandbox boundary.

---

## Goals

### Primary goals

1. Ship a known Python interpreter with desktop builds so users do not need to install or configure Python separately.
2. Make Python behavior reproducible across supported desktop platforms.
3. Keep all model-directed Python execution behind the existing Sandbox Broker.
4. Add a curated scientific/data-analysis package set suitable for spreadsheets, CSV files, charts, lightweight statistics, document transformation, and image processing.
5. Add a higher-level Chat Studio analysis tool with attachment/workspace input and artifact output semantics.
6. Preserve the existing restricted `python_analysis` tier for small calculations and low-complexity analysis.
7. Keep network disabled by default and avoid arbitrary runtime package installation during chat execution.
8. Support desktop and server deployments through one runtime-provider contract while allowing different packaging strategies.
9. Keep bundle provenance, licensing, package inventory, and vulnerability scanning auditable.
10. Maintain truthful sandbox capability reporting and fail closed when the selected deployment cannot supply the required Python environment.

### Secondary goals

- improve the reliability of existing `code_execute(language="python")`;
- allow richer artifact generation, including charts and revised spreadsheets;
- enable future notebook-like and durable analysis-session capabilities without requiring them in the first release;
- provide a foundation for future offline Python-powered first-party skills where Python libraries are materially stronger than available Go libraries.

---

## Non-goals

The initial implementation should **not**:

- embed CPython into the Go process with CGO or a shared-library bridge;
- let arbitrary model code execute directly in the primary Go API process;
- replace the Sandbox Broker with `exec.Command` calls from Chat Studio tools;
- expose the raw host filesystem to Python;
- expose backend environment variables or provider credentials to Python;
- grant general network access to Python;
- support arbitrary `pip install` from PyPI during a chat session;
- turn the primary multi-user server container into an arbitrary tenant execution environment;
- weaken approval, workspace, Git, or artifact boundaries already implemented by OmniLLM-Studio;
- make Python availability a hard requirement for deployments that intentionally disable code execution.

---

## Current-state integration points

The implementation should remain aligned with the current codebase instead of creating parallel abstractions.

### Existing Chat Studio tools

`backend/internal/tools/code_sandbox_tool.go`

- tool name: `code_execute`;
- supports `python`, `javascript`, and `shell`;
- creates/reuses application-issued sandbox sessions;
- executes through `sandbox.Broker.Exec`;
- supports timeout bounds and returned artifacts;
- defaults to no network.

`backend/internal/tools/python_analysis_tool.go`

- tool name: `python_analysis`;
- intentionally restricted Python;
- blocks imports, filesystem access, subprocesses, networking, and dynamic evaluation;
- runs through the shared Sandbox Broker;
- should remain as the lightweight/restricted tier.

### Existing sandbox architecture

The current sandbox program already separates:

- Tool Executor policy and approvals;
- Broker ownership/session/workspace policy;
- runtime capability negotiation;
- platform-native process/filesystem/network enforcement;
- governed workspace mutation;
- guarded Git mutation/publication.

The Python packaging work belongs **below the Broker's language execution contract and above platform process launch**, not beside it.

### Existing roadmap relationship

The sandbox roadmap records Broker-backed `code_execute` and `python_analysis` as implemented, while Linux/runtime packaging remains unfinished. This guide should therefore be treated as the implementation program that completes Python runtime packaging and adds the product-facing rich data-analysis tier on top of the existing execution boundary.

---

## Design principles

### 1. Runtime payload is not the security boundary

`go-embed-python` or an equivalent package may locate/extract Python, but OmniLLM-Studio owns execution security.

Never treat the embedded runtime helper's process-launch API as sandbox enforcement.

### 2. One Broker path

All model-directed Python execution should remain:

```text
Tool -> Sandbox Broker -> selected runtime -> Python
```

No direct host fallback is allowed for a tool advertised as sandboxed Python.

### 3. Reproducible environment

The application should know and expose internally:

- Python version;
- python-build-standalone/runtime build identifier;
- package set and exact versions;
- package-set digest;
- runtime-bundle digest;
- target OS/architecture;
- provenance/license metadata.

### 4. Curated packages, not arbitrary runtime installation

The first releases should use a build-time package manifest. Runtime `pip install` is deferred until there is a governed package-broker design and destination-scoped egress suitable for it.

### 5. Desktop and server packaging may differ

The runtime-provider interface should be common, but packaging should be deployment-specific:

```text
PythonRuntimeProvider
  |-- EmbeddedProvider      Wails desktop
  |-- RootFSProvider        Linux local/runtime image
  `-- ManagedProvider       dedicated server/Kubernetes worker
```

### 6. Preserve two Python tiers

Keep:

- `python_analysis` for restricted calculations and small in-memory processing; and
- a richer sandboxed analysis environment for file-oriented or library-heavy work.

This keeps low-complexity operations small while still enabling advanced workflows.

---

# Phase 0 — Architecture spike and dependency decision

## Objective

Prove that `go-embed-python` is a suitable runtime-distribution dependency for OmniLLM-Studio and define the abstraction that prevents vendor lock-in.

## Work

### Inspect and pin upstream behavior

Evaluate the exact intended `go-embed-python` release for:

- supported platforms;
- embedded Python version;
- python-build-standalone version;
- extraction/cache behavior;
- executable layout;
- bundle size;
- package embedding mechanism;
- update/versioning behavior;
- licenses and notices;
- Windows/macOS signing/notarization implications;
- ARM64 coverage.

### Explicit security rule

Do not use the upstream `PythonCmd` process launch path for model-directed execution. The integration may use APIs that provide the extracted runtime directory or Python executable path, but process creation must remain owned by Omni's sandbox runtime adapters.

### Introduce a provider contract

Proposed new package:

`backend/internal/sandbox/pythonruntime/`

Proposed contract:

```go
type Runtime struct {
    PythonExecutable string
    Home             string
    ReadOnlyRoots    []string
    Version          string
    BundleDigest     string
    PackageDigest    string
}

type Provider interface {
    Prepare(ctx context.Context) (*Runtime, error)
    Metadata(ctx context.Context) (Metadata, error)
    Cleanup(ctx context.Context) error
}
```

Exact naming should follow existing sandbox package conventions after implementation-time inspection.

The provider should return **trusted backend/runtime state only**. Physical interpreter paths must not be surfaced to model-facing APIs.

### Build-size baseline

Record before/after sizes for:

- backend binary;
- Wails Windows package;
- Wails macOS package;
- Linux artifact/container;
- first-run extracted runtime footprint.

## Likely files

- `backend/go.mod`
- `backend/go.sum`
- proposed `backend/internal/sandbox/pythonruntime/*`
- sandbox runtime construction/configuration files discovered during implementation
- build/release scripts under `scripts/`
- CI/dependency audit workflows under `.github/workflows/`
- third-party notice/SBOM files if present

## Exit criteria

- a runtime-provider interface is documented and unit tested;
- a pinned upstream dependency or explicitly documented alternative is selected;
- no tool calls Python through an unsandboxed host process;
- Windows amd64, macOS arm64/amd64, Linux amd64/arm64 compatibility is verified against supported release targets;
- binary/package size impact is measured;
- dependency and licensing review is recorded.

## Risk areas

- large Go binary or installer growth because `//go:embed` contains multiple platform payloads;
- dependency tags not following conventional semver;
- extraction location colliding with sandbox path policy;
- code-signing/notarization behavior for extracted executables;
- unsupported Windows ARM64 if Omni adds that target.

---

# Phase 1 — Embedded Python provider for desktop

## Objective

Make a pinned Python interpreter available to OmniLLM-Studio desktop builds without requiring host Python.

## Work

### Implement provider

Create an embedded provider that:

1. initializes/extracts the pinned Python distribution;
2. verifies the resulting runtime payload before use;
3. returns the interpreter and required read-only runtime roots to the sandbox layer;
4. uses application-controlled cache/extraction directories rather than model-controlled paths;
5. handles concurrent first-run preparation safely;
6. cleans obsolete bundle versions without deleting an active runtime;
7. records runtime/version metadata for diagnostics.

### Configuration

Add configuration with explicit modes, for example:

```text
OMNILLM_PYTHON_RUNTIME_MODE=auto|embedded|external|off
```

Suggested semantics:

- `auto`: use the deployment-default trusted provider;
- `embedded`: require the bundled provider, fail closed if unavailable;
- `external`: use an explicitly configured trusted runtime path/provider;
- `off`: do not advertise rich Python execution.

Do not silently fall back from a requested confined embedded runtime to arbitrary host Python.

### Startup model

Prefer lazy preparation on the first Python execution unless measurements show extraction should occur during install/startup. Avoid blocking normal Chat Studio startup for users who never enable code execution.

## Exit criteria

- fresh desktop install can execute a trivial sandboxed Python program without host Python installed;
- runtime version and digest are deterministic;
- repeated launches reuse a validated extracted runtime;
- corrupt/incomplete extraction fails closed and can recover through controlled re-extraction;
- code execution remains disabled when operator/tool policy disables it;
- no physical runtime path appears in model-visible tool output.

## Validation

- unit tests for provider state and extraction lifecycle;
- clean Windows VM without Python;
- clean macOS runner/machine without relying on Homebrew Python;
- Linux test without system Python in the sandbox rootfs where applicable;
- concurrent initialization test;
- interrupted-extraction recovery test.

---

# Phase 2 — Integrate Python runtime with every platform sandbox

## Objective

Run the embedded interpreter **inside** existing platform-native confinement with exactly the same security semantics as other sandbox processes.

## Common requirements

The sandbox runtime, not the provider, must construct the child environment.

Expected environment should be minimal and explicit, such as:

```text
PYTHONHOME=<sandbox-visible trusted runtime root>
PYTHONPATH=<curated site-package roots if required>
HOME=<runtime-owned home>
TMPDIR/TEMP/TMP=<runtime-owned temp>
PATH=<minimal required paths>
```

Do not inherit `os.Environ()`.

Disable user-site and unsafe ambient Python behavior where appropriate, for example by evaluating controls such as:

```text
PYTHONNOUSERSITE=1
PYTHONDONTWRITEBYTECODE=1
```

The exact environment must be tested per platform because the embedded distribution may require platform-specific runtime variables.

## Linux

Integrate the runtime with the Bubblewrap execution path:

- runtime files mounted read-only;
- sandbox-owned writable home/tmp;
- no network namespace remains the default;
- cgroup PID/memory controls remain applicable;
- project/workspace mounts continue through Broker grants;
- Python bytecode/cache writes must resolve only to writable sandbox-owned locations or be disabled.

## Windows

Integrate with AppContainer/Job execution:

- stage or ACL only the trusted Python distribution using existing confinement principles;
- do not widen ACLs on arbitrary user locations;
- ensure DLL/runtime dependencies remain resolvable inside AppContainer;
- retain zero-network capability by default;
- keep Job Object PID/memory enforcement applicable;
- keep handle inheritance restricted;
- clean staged runtime content/versioned caches safely.

## macOS

Integrate with the Seatbelt runtime:

- explicit read roots for Python executable, stdlib, required libraries, and framework/runtime assets;
- sandbox-owned writable home/tmp;
- default network deny;
- no dynamic path override injection;
- retain current truthful process-tree capability reporting.

## Exit criteria

For each supported platform:

- `python -c 'print(...)'` works through the Broker;
- unrelated host files remain inaccessible;
- ambient backend secrets remain absent;
- network remains denied;
- timeout/cancellation remains enforced;
- available PID/memory quotas remain enforced;
- artifact/workspace authority remains scoped;
- Python runtime roots are read-only to model code;
- capability reporting remains accurate.

## Adversarial tests

Add native negative coverage for:

- modifying interpreter or stdlib files;
- importing from an unrelated host path;
- reading parent application configuration/secrets;
- socket connection attempts;
- spawning descendants beyond declared process limits;
- memory pressure beyond configured limits where supported;
- replacing or symlinking runtime/cache roots;
- environment-variable injection intended to escape runtime roots;
- path traversal through `PYTHONPATH` or working-directory manipulation.

---

# Phase 3 — Curated analysis package bundle

## Objective

Ship a deterministic Python environment capable of useful local data analysis without runtime internet access.

## Initial package profile

A conservative first profile should evaluate and, where support/build size is acceptable, pin:

### Core data

- `numpy`
- `pandas`

### Visualization

- `matplotlib`

### Spreadsheet/document

- `openpyxl`
- `python-docx`

### Image

- `Pillow`

### Optional after size/compatibility review

- `scipy`
- `pyarrow`
- `xlsxwriter`
- PDF-focused packages not duplicating stronger existing Go paths
- other packages justified by concrete Chat Studio workflows

Do not automatically include large packages solely because they are common in notebooks. Every dependency should have a product use case, build-size cost, platform support, license review, and security update owner.

## Package profiles

Consider separate logical profiles even if initially shipped as one bundle:

```text
base
  Python stdlib

analysis
  numpy
  pandas
  matplotlib

office
  openpyxl
  python-docx

image
  Pillow
```

The manifest should allow future deployment profiles to omit unnecessary groups.

## Build-time generation

Create an exact requirements/lock process. Do not rely on floating package versions.

Proposed layout:

```text
backend/internal/sandbox/pythonruntime/
  requirements/
    analysis.txt
    office.txt
    image.txt
  manifest.json
  generate/
```

Generated binary/package data should follow repository conventions and should not make normal source review impractical. If embedding package data through Go causes unreasonable repository/binary churn, prefer release-time generated assets with integrity-pinned manifests while preserving offline install behavior.

## Supply-chain requirements

For every runtime release record:

- source project/version;
- hash/digest;
- license;
- transitive packages;
- known vulnerability scan result;
- Python runtime provenance;
- build timestamp/build identifier.

## Exit criteria

- all selected packages import successfully on supported platforms;
- no package install requires runtime network;
- identical package versions are reported across equivalent releases;
- manifest and digest are available for diagnostics;
- dependency audit/SBOM includes the Python supply chain;
- installer size impact is accepted.

---

# Phase 4 — Harden existing `code_execute` Python behavior

## Objective

Make the existing generic execution tool automatically benefit from the managed Python runtime without changing its core tool contract unnecessarily.

## Work

For `backend/internal/tools/code_sandbox_tool.go` and the downstream sandbox runtime:

- preserve `language: "python"`;
- resolve Python through the runtime provider rather than host PATH;
- preserve session ownership and reuse semantics;
- preserve `ToolArtifact` return behavior;
- attach safe runtime metadata where useful, for example Python/runtime profile version, without exposing host paths;
- return clear failures such as `python runtime unavailable` rather than ambiguous process-start errors;
- keep network metadata truthful;
- keep timeout/result-size limits unchanged unless measured workloads justify explicit changes.

## Compatibility

Existing callers that execute pure-stdlib Python should continue working without modification.

`code_execute` remains generic. Rich file-analysis semantics belong to the dedicated tool in the next phase rather than overloading every `code_execute` invocation.

## Exit criteria

- existing `code_execute` tests remain green;
- Python uses the bundled/provider runtime;
- JavaScript and shell execution paths are unchanged;
- runtime-unavailable behavior is deterministic;
- user/operator tool toggles continue to be respected.

---

# Phase 5 — Add `data_analysis` Chat Studio tool

## Objective

Provide a model-friendly, file-oriented analysis tool that uses the managed Python environment and the existing sandbox/artifact system.

## Proposed tool

Proposed source file:

`backend/internal/tools/data_analysis_tool.go`

Proposed tool name:

`data_analysis`

Alternative name if existing naming conventions favor it:

`python_workspace`

Prefer a semantic name that tells models **when** to choose the tool rather than exposing implementation details alone.

## Proposed tool responsibilities

- accept Python source or an analysis task execution payload;
- attach selected file-library/attachment/workspace inputs through governed references;
- create/reuse an owner-bound analysis sandbox session;
- expose a writable sandbox-owned artifact directory;
- execute through the Broker;
- return stdout/stderr, structured metadata, and generated artifacts;
- never expose host filesystem paths;
- never mutate a governed workspace directly unless using the existing governed mutation APIs.

## Proposed argument shape

Exact schema should match tool conventions, but conceptually:

```json
{
  "code": "...",
  "session_id": "sbx_...",
  "inputs": [
    {"type": "file", "id": "..."},
    {"type": "workspace", "grant_id": "...", "path": "data/source.csv"}
  ],
  "timeout_ms": 30000
}
```

Avoid allowing the model to provide arbitrary physical input paths.

## Artifact contract

Support common outputs such as:

- `.png`, `.jpg`, `.webp` charts/images;
- `.csv`;
- `.xlsx`;
- `.json`;
- `.txt`, `.md`;
- generated office/document files supported by policy;
- other bounded file types accepted by the existing artifact system.

Artifact enumeration must occur from a runtime-owned output boundary, not from arbitrary host paths returned by Python.

## Default execution policy

The tool should likely remain high risk/side-effecting because it executes arbitrary code and creates artifacts, even though host mutation is confined.

Use the existing Tool Executor approval/policy system rather than inventing per-tool approval UI.

## Relationship to `python_analysis`

Tool-selection guidance should make the split clear:

Use `python_analysis` for:

- arithmetic;
- summary statistics;
- small JSON/in-memory transformations;
- calculations not requiring imports/files.

Use `data_analysis` for:

- CSV/XLSX analysis;
- charts;
- multi-step Python analysis;
- document/image transformations;
- curated third-party libraries;
- generated artifacts.

## Exit criteria

- tool is registered only when the managed runtime/profile is available and operator policy permits it;
- model can analyze a CSV and return summary + chart artifact;
- model can read XLSX and generate a revised XLSX artifact;
- input paths remain reference/grant based;
- generated artifacts are owner-scoped and bounded;
- no unrestricted filesystem/network access is introduced;
- failure messages are visible to Chat Studio rather than silently swallowed.

---

# Phase 6 — File Library and attachment integration

## Objective

Make uploaded files easy for Chat Studio to analyze without exposing their physical storage layout.

## Work

Introduce or reuse an application-owned staging flow:

```text
File Library / chat attachment ID
        |
        v
owner/workspace authorization
        |
        v
bounded read-only staging or runtime mount
        |
        v
sandbox-visible logical path
```

Requirements:

- validate owner/workspace access before staging;
- reject unsupported/suspicious special files;
- enforce per-input and aggregate byte limits;
- preserve original filename separately from trusted physical storage name;
- avoid symlink/reparse/hard-link escapes;
- pass logical sandbox paths to the generated execution request, never host paths;
- clean staged inputs with session/task lifecycle;
- record source attachment IDs in execution metadata for auditability.

## Useful first workflows

1. CSV exploration and aggregation.
2. XLSX analysis with charts and revised workbook output.
3. JSON transformation.
4. Text corpus analysis.
5. Basic image metadata/transformation with Pillow.
6. Multi-file comparison where aggregate limits permit it.

## Exit criteria

- Chat Studio can invoke analysis directly against an uploaded file;
- authorization is revalidated at execution time;
- deleted/revoked files fail closed;
- staged files are read-only;
- physical storage roots never appear in model context;
- cleanup is proven.

---

# Phase 7 — Chat Studio UX and tool controls

## Objective

Expose the capability consistently with existing Chat Studio tool controls and make executions understandable to users.

## Backend

Update tool registry/composition so availability depends on:

- operator configuration;
- runtime provider readiness;
- required sandbox capabilities;
- existing per-user/workspace tool policy.

Do not advertise `data_analysis` if the runtime is not actually usable.

## Frontend

Likely integration areas, after implementation-time verification:

- Chat Studio tool Settings panel;
- tool-definition/types used by the frontend;
- tool-call rendering;
- artifact cards/download/open actions;
- error/progress state;
- optional runtime capability/status diagnostics.

### Suggested UX label

**Data Analysis**

Description:

> Analyze spreadsheets, CSVs, data, images, and documents with a sandboxed local Python environment. Can create charts and downloadable output files.

### Execution presentation

Show:

- that Data Analysis is running;
- code/tool-call details according to existing disclosure settings;
- generated artifacts;
- explicit execution errors;
- cancellation if the existing execution-cancel path is exposed to Chat Studio;
- sandbox/network state where current UI conventions already expose it.

## Exit criteria

- user can turn the tool on/off using the existing tool-control system;
- disabled tool is not offered to the model;
- execution state and failures render reliably;
- charts/files appear through standard artifact UI;
- no new parallel permissions/settings system is introduced.

---

# Phase 8 — Analysis quality and model tool-selection behavior

## Objective

Ensure models choose the appropriate compute tool and use the richer runtime effectively.

## Work

### Tool descriptions

Tune tool descriptions/examples so models understand:

- `python_analysis` = restricted, small in-memory calculation;
- `data_analysis` = files, libraries, charts, generated outputs;
- `code_execute` = general sandboxed code when semantic analysis tooling is not a better fit.

### Representative eval set

Add evaluation cases such as:

1. calculate mean/median from inline values -> `python_analysis`;
2. analyze an uploaded CSV -> `data_analysis`;
3. create a bar chart -> `data_analysis`;
4. revise an XLSX workbook -> `data_analysis`;
5. run a small Python algorithm with no files -> `code_execute` or `python_analysis` depending on policy;
6. request live web/API data from Python -> reject/avoid because sandbox network is disabled; use existing network-aware tools instead;
7. request arbitrary package installation -> explain unavailable or use curated alternatives;
8. ask to edit project source -> governed workspace/Git tools, not unrestricted Python mutation.

### Error recovery

Validate behavior for:

- missing package;
- malformed spreadsheet;
- memory limit exceeded;
- timeout;
- revoked input;
- unsupported artifact type;
- output too large;
- interrupted execution.

## Exit criteria

- evals show reliable tool selection;
- tool failures return actionable feedback to the model/user;
- models do not habitually choose rich analysis for trivial arithmetic;
- models do not attempt Python network work when existing web/API tools are the correct path.

---

# Phase 9 — Desktop packaging and release engineering

## Objective

Make the capability supportable across OmniLLM-Studio desktop releases.

## Work

### Wails packaging

Validate:

- Windows amd64;
- macOS arm64;
- macOS amd64 if still shipped;
- any Linux desktop target currently supported.

### Extraction/cache location

Use application-owned per-user cache/state directories with:

- versioned runtime directories;
- integrity marker/digest;
- atomic extraction finalization;
- stale-version cleanup;
- no dependence on current working directory;
- no user/model-selectable extraction root.

### Signing/notarization

Explicitly prove that the selected packaging/extraction model works with:

- Windows application signing and runtime execution policy;
- macOS signing/notarization/Gatekeeper behavior;
- antivirus/EDR behavior likely to flag newly extracted executable payloads.

If extracted executable signing is operationally problematic, evaluate shipping the runtime as signed application resources beside the binary instead of embedding/extracting it dynamically. The provider abstraction must allow that packaging change without changing Chat Studio tool APIs.

### Updates

Application update should:

- install the new runtime version atomically;
- preserve active sessions until safe cleanup;
- remove old inactive runtime versions;
- never replace an executing runtime in place.

## Exit criteria

- clean signed installers execute Python successfully;
- first-run extraction/preparation is reliable;
- application upgrades across Python bundle versions are reliable;
- rollback does not leave an incompatible package/runtime mixture;
- disk cleanup is bounded and tested.

---

# Phase 10 — Server and Kubernetes execution strategy

## Objective

Support rich Python analysis in multi-user deployments without embedding arbitrary tenant execution into the primary API process/container.

## Dependency

This phase should align with the sandbox roadmap's dedicated server/Kubernetes worker phase.

## Architecture

Prefer:

```text
Primary API
    |
    v
Sandbox Broker / task admission
    |
    v
Dedicated execution worker
  - pinned Python image/runtime
  - curated packages
  - non-root
  - read-only root filesystem where practical
  - ephemeral scratch
  - resource limits
  - NetworkPolicy/default deny
  - owner/task isolation
```

For worker/container deployments, a preinstalled Python runtime may be preferable to `go-embed-python`. The application-level `PythonRuntimeProvider` remains useful because the tool/Broker contract does not change.

## Requirements

- no arbitrary tenant Python in the primary API container;
- worker identity isolated from backend credentials;
- explicit CPU/memory/PID/ephemeral-disk quotas as platform support matures;
- default-deny network;
- owner/workspace/task association;
- bounded artifact return path;
- safe task cancellation and worker cleanup;
- Kubernetes security context aligned with current sandbox roadmap.

## Exit criteria

- same `data_analysis` tool contract works in desktop and server modes;
- deployment reports actual runtime capabilities;
- worker unavailable -> tool unavailable/fail closed;
- tenant isolation and cleanup have adversarial evidence.

---

# Phase 11 — Optional durable analysis sessions

## Objective

Improve multi-turn workflows after the basic tool is stable.

This phase is optional and should follow the sandbox roadmap's durable-task architecture rather than inventing a Python-only scheduler.

Potential capabilities:

- conversation-bound analysis session reuse;
- persisted logical input/artifact references;
- controlled session TTL extension;
- durable task recovery;
- execution history and provenance;
- notebook-like cell history in Chat Studio;
- reusable intermediate data stored inside an owner-bound session scratch volume.

Do not persist arbitrary live Python process state as the first design. Prefer recoverable task/session state with explicit artifacts and replayable code where practical.

## Exit criteria

- sessions are owner/conversation bound;
- expiration/cleanup is deterministic;
- server restart behavior is documented;
- persisted references do not become authorization;
- task recovery aligns with general sandbox task architecture.

---

# Phase 12 — Governed package extension, if justified

## Objective

Provide additional packages without granting arbitrary Python internet access.

This is explicitly deferred.

Possible future model:

```text
requested package
      |
      v
approved package policy/catalog
      |
      v
application package broker
  - exact name/version
  - allowlist/license policy
  - malware/vulnerability checks
  - artifact hash
      |
      v
cached immutable wheel/package layer
      |
      v
sandbox read-only package mount
```

Do **not** implement this by simply enabling `pip install` with unrestricted egress.

Dependencies include:

- destination-scoped egress enforcement or trusted backend-side download;
- credential separation;
- package integrity/signature/hash policy;
- disk quotas/cleanup;
- package-layer provenance;
- administrator controls.

---

# Security requirements

The implementation is incomplete unless the following remain true.

## Process boundary

- Python never runs inside the Go API process.
- All model-directed execution passes through the Broker.
- Tool callers cannot substitute an arbitrary executable for the trusted Python provider.

## Environment

- no wholesale `os.Environ()` inheritance;
- provider/API keys are absent;
- cloud credentials are absent;
- GitHub credentials are absent unless a future service-specific broker intentionally supplies scoped authority;
- proxy/auth delegation variables remain blocked according to existing sandbox policy.

## Filesystem

- Python runtime is read-only;
- inputs are read-only unless explicitly modeled otherwise;
- scratch/output is sandbox owned;
- physical host paths are backend-private;
- no direct arbitrary host workspace mutation;
- governed workspace mutation remains application mediated.

## Network

- no-network remains the default;
- `network_allowlist` is not advertised merely because a Python library can make HTTP requests;
- future egress must use the sandbox network-grant design and real destination enforcement.

## Lifecycle

- timeout kills execution according to runtime capability;
- cancellation remains addressable;
- session destruction cleans Python descendants as strongly as the selected platform truthfully supports;
- extracted runtime caches are not owned by the sandbox process.

## Resource control

- preserve Broker fail-closed admission for requested unsupported quotas;
- Windows Job Object PID/memory limits apply to Python descendants;
- Linux cgroup PID/memory limits apply when advertised;
- macOS must not claim unsupported resource controls;
- output/artifact size remains bounded.

## Supply chain

- runtime and package versions pinned;
- hashes recorded;
- SBOM/third-party notices updated;
- vulnerability scanning covers Python dependencies;
- Python bundle upgrades occur through deliberate PR/release work, not automatic semver assumptions.

---

# Data and artifact model

## Inputs

Preferred identifiers:

- File Library ID;
- chat attachment ID;
- workspace grant ID + relative path;
- existing artifact ID when re-analysis is permitted.

Never accept a model-provided physical host path as an authorization mechanism.

## Outputs

Generated output should become standard OmniLLM artifacts with:

- owner scope;
- conversation/run association where available;
- MIME type;
- byte size;
- digest;
- safe filename;
- source execution ID;
- runtime/package profile metadata;
- expiration/persistence behavior matching existing artifact policy.

## Provenance

Useful structured execution metadata:

```json
{
  "runtime": "local_windows_appcontainer",
  "python_version": "3.x.y",
  "python_profile": "analysis-v1",
  "python_bundle_digest": "...",
  "execution_id": "...",
  "session_id": "sbx_...",
  "network": "none",
  "input_ids": ["..."],
  "artifact_ids": ["..."]
}
```

Do not include trusted physical paths.

---

# Package-version management

Because `go-embed-python` release tags encode Python/python-build-standalone/build information rather than conventional semantic-version progression, do not allow routine dependency automation to change the embedded runtime implicitly.

Recommended policy:

1. pin the exact module tag;
2. ignore or separately group this dependency in automated update tooling;
3. update it only through a dedicated runtime-upgrade PR;
4. include package manifest and bundle digest changes in that PR;
5. run the full native Python sandbox matrix before merge.

A Python runtime upgrade should be treated more like a runtime image upgrade than an ordinary Go library patch.

---

# Suggested feature flags and configuration

Names are proposed and should be reconciled with existing config conventions before implementation.

```text
OMNILLM_CODE_EXEC_ENABLED=true|false
OMNILLM_PYTHON_RUNTIME_MODE=auto|embedded|external|off
OMNILLM_PYTHON_RUNTIME_PATH=<trusted operator path when external>
OMNILLM_DATA_ANALYSIS_ENABLED=true|false
OMNILLM_PYTHON_PROFILE=analysis-v1
```

Avoid a large set of user-tunable low-level Python path variables. Runtime roots should be operator/backend state, not model/user-controlled settings.

---

# Testing strategy

## Unit tests

Add tests around:

- provider selection;
- runtime metadata/digests;
- corrupt-cache recovery;
- tool registry availability;
- argument validation;
- artifact enumeration;
- input authorization;
- package manifest validation;
- no-host-fallback behavior.

## Integration tests

Representative Python programs:

```python
print("hello")
```

```python
import numpy as np
print(np.mean([1, 2, 3]))
```

```python
import pandas as pd
print(pd.DataFrame({"a": [1, 2]}).describe().to_json())
```

```python
import matplotlib.pyplot as plt
plt.plot([1, 2, 3])
plt.savefig("/artifacts/chart.png")
```

Exact sandbox artifact path/API should follow runtime conventions rather than hard-coding this example if the current runtime uses another output root.

## Native security tests

For every supported desktop platform:

- secret environment absence;
- host file denial;
- runtime-file write denial;
- network denial;
- timeout;
- cancellation;
- descendant lifecycle;
- PID/memory enforcement where advertised;
- runtime-root replacement/symlink/reparse attacks;
- unsafe Python path/environment injection.

## Frontend/E2E tests

Playwright scenarios:

1. Data Analysis appears when available/enabled.
2. Disabling the tool removes it from effective Chat Studio tools.
3. CSV analysis renders a successful tool result.
4. chart artifact appears and can be opened/downloaded through existing UI.
5. XLSX output artifact appears.
6. Python exception is visible to the user.
7. timeout/cancellation state is visible.
8. runtime unavailable produces an actionable error rather than a silent failed tool call.

## Eval tests

Measure:

- correct tool selection;
- analysis correctness;
- artifact completion;
- retry behavior after Python exceptions;
- avoidance of unsupported network/package-install behavior.

---

# Performance and capacity targets

Establish baselines before setting hard targets.

Measure:

- first Python call after fresh install;
- warm Python startup;
- first import of NumPy/pandas/matplotlib;
- 10 MB CSV load/summary;
- representative XLSX read/write;
- chart generation;
- first extraction duration;
- extracted runtime disk footprint;
- compressed installer contribution;
- memory footprint by package profile.

Optimize only after measurements. Likely levers include:

- lazy runtime extraction;
- shared immutable extracted runtime across sessions;
- session reuse through `sbx_...` ownership scope;
- smaller package profile;
- precompiled Python bytecode only if it does not create portability/update problems;
- server-side preinstalled runtime layers.

Do not reuse writable Python environments across owners.

---

# Observability and diagnostics

Add safe diagnostics sufficient to answer:

- Is a Python runtime configured?
- Which provider is selected?
- Which Python/profile version is active?
- Is runtime preparation healthy?
- What runtime capability rejected a request?
- Was a failure interpreter startup, user code, resource limit, cancellation, or artifact collection?

Do not log:

- model input files in full;
- provider secrets;
- sanitized environment values that may still contain sensitive data;
- trusted host paths unless existing debug logging policy explicitly permits and redacts them.

Useful metrics may include:

- runtime preparation success/failure;
- Python execution count;
- execution duration;
- timeout/cancellation count;
- resource-limit terminations;
- generated artifact count/bytes;
- tool error classification.

---

# Documentation updates required during implementation

At minimum review/update:

- `README.md` if user-visible installation/runtime behavior changes;
- `CLAUDE.md` / contributor instructions if build generation steps change;
- `docs/Feature FAQ.md`;
- current sandbox architecture document;
- current sandbox roadmap;
- threat model if new trust surfaces are introduced;
- deployment docs for Docker/Kubernetes/server behavior;
- desktop build docs/scripts;
- third-party notices/SBOM documentation;
- tool documentation/settings help text.

This guide should remain the durable implementation record until the work is complete. Each merged phase should update its status and evidence rather than leaving completion implicit in commit history.

---

# Proposed phase status tracker

| Phase | Scope | Initial status | Exit evidence |
|---|---|---:|---|
| 0 | Architecture spike + dependency decision | NOT STARTED | provider contract, size/security/license decision |
| 1 | Embedded desktop Python provider | NOT STARTED | host-Python-free desktop execution |
| 2 | Linux/Windows/macOS sandbox integration | NOT STARTED | native positive + negative runtime evidence |
| 3 | Curated analysis package bundle | NOT STARTED | pinned manifest, imports, SBOM, size approval |
| 4 | `code_execute` Python hardening | NOT STARTED | existing contract uses managed Python |
| 5 | `data_analysis` tool | NOT STARTED | CSV/XLSX/chart artifact workflows |
| 6 | File Library + attachment integration | NOT STARTED | owner-scoped staged analysis inputs |
| 7 | Chat Studio UX/tool controls | NOT STARTED | settings + tool-call/artifact UX |
| 8 | Tool-selection evals and recovery | NOT STARTED | model-selection/error eval suite |
| 9 | Desktop packaging/release engineering | NOT STARTED | signed clean-install/update evidence |
| 10 | Server/Kubernetes worker strategy | NOT STARTED | dedicated-worker deployment evidence |
| 11 | Durable analysis sessions | DEFERRED | general durable-task integration |
| 12 | Governed package extension | DEFERRED | package-broker/egress architecture |

---

# Recommended PR sequence

Keep implementation slices small and independently reviewable.

### PR A — Runtime provider abstraction and spike

- provider interface;
- pinned dependency decision;
- metadata/digest model;
- extraction tests;
- no production tool routing change unless needed to prove the provider.

### PR B — Desktop embedded runtime + `code_execute` Python

- embedded provider;
- sandbox-visible runtime mapping;
- current Python code execution uses managed runtime;
- native Windows/macOS/Linux tests as applicable;
- no third-party analysis packages yet.

### PR C — Curated package profile

- manifest/generator;
- NumPy/pandas/matplotlib first;
- supply-chain metadata;
- package import tests;
- installer-size evidence.

### PR D — `data_analysis` backend tool

- tool definition;
- owner-bound sandbox execution;
- input/artifact model;
- CSV and chart integration tests;
- no frontend redesign yet beyond required API/type wiring.

### PR E — File Library/attachment staging

- reference-based input staging;
- authorization and cleanup;
- XLSX workflow;
- security negatives.

### PR F — Chat Studio UX and E2E

- settings/tool enablement;
- execution UI;
- artifacts;
- failure/cancellation handling;
- Playwright coverage.

### PR G — Packaging/release hardening

- Wails installer behavior;
- signing/notarization tests;
- cache/update cleanup;
- documentation.

Server worker support should stay in a separate program/PR series aligned with Sandbox Phase 15.

---

# Release gates

The feature should not be declared complete until all applicable gates pass.

## Functional

- no host Python required for supported desktop build;
- restricted `python_analysis` remains functional;
- `code_execute` Python remains functional;
- `data_analysis` handles CSV/XLSX/chart scenarios;
- artifacts round-trip through Chat Studio.

## Security

- no ambient secrets;
- no unrestricted host filesystem;
- no unexpected network;
- runtime/package roots immutable to sandbox code;
- workspace references reauthorized;
- resource/cancellation controls retain current guarantees;
- no direct process launch from Chat Studio outside Broker.

## Cross-platform

- Windows native sandbox job green;
- macOS native sandbox job green;
- Linux native sandbox job green;
- Wails build/package lanes green;
- container builds remain green if they include the dependency.

## Repository quality

After implementation-time confirmation of available scripts:

```bash
cd backend && gofmt -w <changed-go-files>
cd backend && go vet ./...
cd backend && go test ./...
```

Run frontend lint/build/test and Playwright commands exactly as defined by the current `frontend/package.json`, root `package.json`, and CI workflows at the time of each PR.

## Supply chain

- Go dependency audit green;
- Python dependency scan green;
- CodeQL/security scan green where applicable;
- SBOM/notice updates present;
- exact Python runtime upgrade intentional and documented.

---

# Decision points that require evidence

The following should be answered by Phase 0/1 measurements rather than assumption:

1. Is `go-embed-python` the final dependency, or should Omni copy the same architectural pattern with release-managed platform assets?
2. Does embedding all supported distributions into the Go module create unacceptable binary/repository growth?
3. Does macOS notarization reliably permit the extracted interpreter payload model?
4. Does Windows signing/AV/EDR tolerate first-run extracted Python executables?
5. Which analysis packages provide enough product value to justify bundle size?
6. Should desktop package profiles be one bundle or optional downloadable offline-verified bundles?
7. Is analysis session reuse materially faster than one-shot execution without complicating cleanup?

If any of these fail, retain the `PythonRuntimeProvider` abstraction and change only the packaging provider. The Chat Studio, Broker, sandbox, and data-analysis contracts should remain stable.

---

# Final target state

When the program is complete, a user should be able to install OmniLLM-Studio on a clean supported desktop with no Python installation, enable **Data Analysis**, attach a spreadsheet, and ask:

> Analyze this workbook, identify the weakest region, create a chart, and give me a revised workbook with a summary tab.

The execution should follow:

```text
uploaded workbook
      |
      v
owner-authorized read-only input
      |
      v
Chat Studio data_analysis tool
      |
      v
Sandbox Broker
      |
      v
platform-native confined managed Python
  + pinned analysis packages
  + no network
  + bounded resources
      |
      v
chart.png + revised.xlsx
      |
      v
owner-scoped Omni artifacts
      |
      v
Chat Studio response
```

The user receives a rich local data-analysis experience while the architectural guarantees developed by the sandbox program remain authoritative.

---

## Recommended next action

Start with **PR A: Runtime provider abstraction and dependency spike**. Do not begin by wiring `go-embed-python` directly into `code_execute`. The first implementation slice should establish the runtime-provider seam, document exact dependency/version/provenance, measure bundle impact, and prove that every eventual Python child will still be launched by the existing platform sandbox runtimes with a sanitized environment.
