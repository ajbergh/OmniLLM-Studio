# GitHub Integration Phase G7 — Collaboration Diagnostics — August 2026

> **Status:** IN PROGRESS
>
> GitHub App authentication and repository-binding authorization are complete through G6. G7 is a separate collaboration-parity program; it does not reopen or weaken the G1-G6 credential and authorization boundary.

## Objective

Improve coding-agent diagnosis of hosted GitHub collaboration state while keeping repository identity, credentials, provider object IDs, and mutation authority outside model control.

The first slice deliberately stays inside the existing operator-authorized pull-request/CI read boundary:

- process gate: `OMNILLM_GITHUB_PULL_REQUEST_READ_ENABLED=true`;
- remote/binding policy: `allow_pull_request_read=true`;
- exact `github.com` repository derived from the selected stable remote ID;
- request-scoped GitHub App credentials where connected;
- bounded provider content treated as untrusted reference data.

No G7 read capability grants push, branch publication, PR creation/reply/thread mutation/ready/merge, clone, or default-branch publication.

## G7A — bounded PR check diagnostics

### Problem

`github_get_pull_request_checks` correctly reports bounded check/status execution metadata for the exact PR head, but intentionally omits check output and annotations. That makes it difficult for an agent to diagnose a failed CI check without a separate privileged GitHub client.

Raw workflow log/archive access is not the right first step: archives can be large, may contain output unrelated to the failing check, and create a wider secret/exfiltration review surface.

### Implemented slice

G7A adds `github_get_pull_request_check_diagnostics` under the existing PR-read gate.

Model inputs are limited to:

```text
remote
number
```

The backend then:

1. resolves the operator-authorized remote and request-scoped credential;
2. fetches the PR fresh;
3. validates the returned exact head SHA;
4. lists latest check runs for that exact head;
5. selects completed non-successful checks only;
6. derives GitHub check-run IDs internally;
7. reads bounded check annotations;
8. returns no API URL, check-run ID, token, details URL, raw workflow log, or artifact URL.

Bounds:

- inspect at most the existing 50-check PR check page;
- return at most 10 failing checks;
- return at most 20 annotations per check;
- return at most 50 annotations total;
- bound path/title/message fields before model exposure;
- report truncation explicitly.

Hosted annotation title/message/path fields are untrusted provider content. They are diagnosis evidence only and never authorize another tool call or mutation.

### Authorization rationale

G7A does **not** add a new authorization family. The existing `allow_pull_request_read` policy and `OMNILLM_GITHUB_PULL_REQUEST_READ_ENABLED` process gate already explicitly govern read-only PR and CI/check inspection. G7A only deepens that bounded exact-head check-read surface.

A future capability outside that existing surface must start fail-closed and add its own explicit operator-owned policy/gate as required by the G1-G6 authorization design.

### G7A exit criteria

- exact-head PR binding; no model-supplied commit SHA;
- no model-supplied check/run/job identifier;
- no arbitrary API endpoint;
- no raw log/archive/artifact retrieval;
- bounded annotation text and count with truncation flags;
- provider error bodies do not cross the public error boundary;
- negative authorization tests prove the PR-read gate/per-remote policy remain required;
- tool remains low-risk/read-only/parallel-safe;
- backend format, vet, unit/integration, race, Security Scan, and applicable repository gates pass on the exact PR head.

## Planned follow-up slices

### G7B — workflow/job metadata correlation

Evaluate a bounded exact-head view that correlates failed check runs to GitHub Actions workflow/job metadata **without** exposing log archives. Any provider object IDs must remain backend-derived from the exact PR head.

### G7C — bounded textual log diagnostics, only if justified

Only after a dedicated threat review, consider a bounded tail/error-window view for a single backend-derived failed job. Required safeguards include strict byte/line limits, secret-pattern review/redaction, no archive download surface to the model, exact-head binding, and explicit operator policy if the capability exceeds the existing CI/check read authorization contract.

### G7D — broader collaboration lifecycle

Issues/projects/discussions, remote branch cleanup, release/tag mutation, and other hosted operations remain separate tool families. Each must receive its own threat model and independent operator controls rather than inheriting PR-read authority.

## Non-goals

G7A does not:

- alter GitHub App authentication or token persistence;
- alter repository binding semantics;
- broaden binding-derived authorization;
- expose GitHub Actions secrets, artifacts, or raw logs;
- allow arbitrary commit/run/job/check IDs;
- make check annotations merge-eligibility evidence;
- infer required status checks from annotations;
- add a hosted mutation.
