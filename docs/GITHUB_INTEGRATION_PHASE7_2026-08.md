# GitHub Integration Phase G7 — Collaboration Diagnostics — August 2026

> **Status:** G7A and G7B merged in PRs #161 and #163; G7C threat review complete and raw workflow-log access remains deferred
>
> GitHub App authentication and repository-binding authorization are complete through G6. G7 is a separate collaboration-parity program; it does not reopen or weaken the G1-G6 credential and authorization boundary.

## Objective

Improve coding-agent diagnosis of hosted GitHub collaboration state while keeping repository identity, credentials, provider object IDs, and mutation authority outside model control.

The current read slices deliberately stay inside the existing operator-authorized pull-request/CI read boundary:

- process gate: `OMNILLM_GITHUB_PULL_REQUEST_READ_ENABLED=true`;
- remote/binding policy: `allow_pull_request_read=true`;
- exact `github.com` repository derived from the selected stable remote ID;
- request-scoped GitHub App credentials where connected;
- bounded provider content treated as untrusted reference data.

No G7 read capability grants push, branch publication, PR creation/reply/thread mutation/ready/merge, clone, or default-branch publication.

## G7A — bounded PR check diagnostics — merged in PR #161

`github_get_pull_request_check_diagnostics` deepens the existing exact-head check read surface without exposing raw workflow logs or model-controlled provider object IDs.

Model inputs are limited to:

```text
remote
number
```

The backend:

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

G7A's exact final PR head passed the repository Quality Gate, Security Scan, applicable container validation, and final review-thread check before merge.

## G7B — bounded workflow/job status correlation — merged in PR #163

### Problem

Check-run annotations identify many failures, but a coding agent also needs to understand which GitHub Actions workflow, job, and step failed or is still running. Accepting arbitrary workflow-run/job IDs would weaken the exact-PR-head binding established in G7A, while raw logs would materially expand the content and secret-exposure surface.

### Implemented slice

G7B adds `github_get_pull_request_workflow_jobs` under the same existing PR/CI read authorization boundary.

Model inputs remain limited to:

```text
remote
number
```

The backend then:

1. resolves the authorized remote and request-scoped credential;
2. fetches the PR fresh and validates its exact head SHA;
3. queries GitHub Actions workflow runs with that exact `head_sha` filter;
4. independently rejects any returned workflow run whose provider-side `head_sha` differs from the freshly fetched PR head;
5. derives workflow-run IDs internally;
6. retrieves bounded job/step **status metadata** for those derived runs;
7. derives job IDs internally and does not expose them;
8. returns no API URL, run ID, job ID, token, runner name, logs, artifacts, command output, or job output.

Returned metadata is limited to bounded names, status/conclusion, workflow run number/attempt, timestamps, and step numbers.

Bounds:

- at most 10 workflow runs;
- at most 20 jobs per workflow run;
- at most 50 jobs total;
- at most 30 steps per job;
- at most 200 steps total;
- explicit run/job/step truncation flags.

If the global job bound prevents remaining workflow runs from being traversed, both job and run truncation are reported so callers cannot mistake the partial result for a complete workflow list.

### GitHub App provider permission prerequisite

The existing OmniLLM authorization boundary remains `OMNILLM_GITHUB_PULL_REQUEST_READ_ENABLED` plus `allow_pull_request_read`. Separately, GitHub itself must grant the request credential sufficient **Actions read** permission for the workflow-run/job REST endpoints.

This provider permission is not inferred from a repository binding and is not silently escalated by G7B. If the connected credential lacks it, the tool fails closed with a sanitized inspection error; GitHub response bodies do not cross the public error boundary.

### Authorization rationale

G7B remains a read-only PR/CI diagnostic capability and adds no hosted mutation authority. Repository selection, exact PR head, workflow-run IDs, job IDs, API host, and credentials all remain outside model control.

### G7B exit criteria

- exact-head PR binding and provider-side head revalidation;
- no model-supplied commit SHA, workflow-run ID, job ID, API URL, or token;
- no provider run/job IDs exposed in tool output;
- no runner names, logs, artifacts, command text, or output text exposed;
- strict count/text bounds and truthful truncation flags;
- provider error bodies do not cross the public error boundary;
- negative authorization tests prove the PR-read gate/per-remote policy remain required;
- tool remains low-risk/read-only/parallel-safe;
- exact final PR head passes Quality Gate, Security Scan, applicable container validation, and final review/diff checks.

G7B's exact final PR head met these criteria before merge as `7e3516cfbefa551a6889d336774d71eac299ac29`.

## G7C — raw textual CI log threat review — implementation deferred

The dedicated review is recorded in `GITHUB_INTEGRATION_PHASE7C_LOG_THREAT_REVIEW_2026-08.md`.

The current decision is **not** to expose raw GitHub Actions job logs under the G7A/G7B PR-read authority.

Reasons:

- logs are arbitrary repository/action output and can disclose transformed credentials or sensitive non-secret data;
- raw log transport can require download/redirect behavior that would expand the deliberately fixed-host, no-redirect GitHub API boundary;
- logs are large/unbounded compared with annotations and status metadata;
- log text is a prompt-injection/control-confusion surface and cannot be merge/authorization evidence;
- provider masking is not a sufficient OmniLLM redaction policy;
- G7A annotations plus G7B workflow/job/step metadata should be evaluated before accepting the larger risk surface.

A future G7C code slice is conditional on an independent workflow-log authorization gate/per-remote policy, provider-permission documentation, a dedicated safe redirect/download transport, independent compressed/decoded/result bounds, pre-model redaction with adversarial tests, exact-head/backend-derived job binding, and untrusted-content typing.

The shared GitHub API client's redirect prohibition remains unchanged.

## Planned follow-up slice

### G7D — broader collaboration lifecycle

Issues/projects/discussions, remote branch cleanup, release/tag mutation, workflow re-runs/cancellation, and other hosted operations remain separate tool families. Each must receive its own threat model and independent operator controls rather than inheriting PR-read authority.

Safer next parity work should prioritize bounded collaboration metadata/actions with explicit independent authorization families instead of weakening the CI content boundary.

## Non-goals

G7A/G7B do not:

- alter GitHub App authentication or token persistence;
- alter repository binding semantics;
- broaden binding-derived authorization;
- expose GitHub Actions secrets, artifacts, raw logs, runner names, or command output;
- allow arbitrary commit/run/job/check IDs;
- make check annotations or workflow metadata merge-eligibility evidence;
- infer required status checks from annotations/workflow metadata;
- rerun/cancel workflows or jobs;
- add a hosted mutation.

G7C does not change those non-goals; it records why raw log access remains deferred until a separate guarded design exists.
