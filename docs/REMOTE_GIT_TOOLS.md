# Remote Git tools — Phase 3

Phase 3 extends the guarded local Git capabilities in `docs/LOCAL_GIT_TOOLS.md` to remote repositories. Remote Git is treated as a separate security boundary because it combines outbound network access, credentials, repository mutation, and potentially destructive remote side effects.

## Security model

Remote endpoints are operator configuration, not model arguments. This is mandatory because tool arguments are persisted in the tool-invocation audit trail. A model-facing call will use only stable remote IDs and state preconditions; it will never receive or submit a raw remote URL, credential value, or credential environment-variable name.

The initial configuration format is `OMNILLM_GIT_REMOTES_JSON`, a JSON object keyed by stable remote ID:

```json
{
  "omnillm-origin": {
    "repository": "omni",
    "url": "https://github.com/ajbergh/OmniLLM-Studio.git",
    "username": "git",
    "token_env": "OMNILLM_GIT_TOKEN_GITHUB",
    "allow_push": true
  }
}
```

The token value lives only in the named operator environment variable. It is loaded immediately before authentication and is not returned by remote inventory/status results or included in tool arguments.

Two independent process-level gates are reserved:

- `OMNILLM_GIT_REMOTE_ENABLED=true` — permits outbound remote Git operations.
- `OMNILLM_GIT_REMOTE_PUSH_ENABLED=true` — additionally permits push when the selected remote also has `allow_push: true`.

The existing tool approval policy remains another independent gate. Network mutations and push will be high-risk, side-effecting tools that default to `ask`.

## Egress boundary

The first implementation slice accepts only exact configured HTTPS endpoints on the normal HTTPS port. It rejects URL credentials, query strings, fragments, HTTP, SSH, `git://`, `file://`, alternate ports, private/loopback/link-local/reserved/metadata IP ranges, environment proxies, and redirects.

DNS is resolved once and every returned address is validated before dialing one validated IP directly. The remote-status transport is dedicated to Git and does not replace `http.DefaultTransport` or go-git's global protocol registry. Advertised-reference responses are bounded to 4 MiB.

Private-network Git servers are deliberately unsupported in this first slice. Supporting them later requires an explicit destination allowlist rather than a blanket private-network bypass.

## Planned tool surface

Phase 3 is being implemented in reviewable slices:

1. **Foundation (current slice)** — remote configuration, credential indirection, HTTPS egress guard, and a bounded remote-ref inspection service. Nothing in this slice is registered as a model-facing tool yet.
2. **Remote inspection tool** — register `git_remotes` and approval-gated `git_remote_status` after CI/CodeQL validates the foundation.
3. **Guarded fetch** — fetch only from the configured remote bound to the configured local repository. No arbitrary URLs, force, prune, submodule recursion, or worktree checkout. Fetch approval will carry explicit local/remote state preconditions.
4. **Guarded push** — current local branch only; no arbitrary refspecs, tags, deletes, force, or history rewriting. Push requires local branch/HEAD/index state plus the expected remote branch head observed during remote inspection. The implementation will also use go-git's server-side `RequireRemoteRefs` guard so a remote branch changing after approval fails closed.
5. **Clone** — exposed only after a hard storage-admission design can prevent a hostile or unexpectedly large remote from exhausting the configured storage budget during transfer. A free-space preflight plus post-clone size check is not considered sufficient by itself.

## Push invariants

The eventual `git_push` contract will require all of the following:

- remote network gate enabled;
- push gate enabled;
- selected configured remote has `allow_push: true`;
- high-risk user approval;
- repository is on a local branch, not detached HEAD;
- expected local branch and HEAD still match the approved state;
- staged index/worktree policy is explicit rather than inferred;
- destination branch is derived from the approved current local branch, not an arbitrary model refspec;
- expected remote branch head still matches the previously inspected value;
- a non-force fast-forward update;
- server-side remote-ref precondition through go-git `PushOptions.RequireRemoteRefs`;
- no delete, force, force-with-lease override, prune, follow-tags, mirror, or arbitrary push options.

## Clone quota requirement

Clone can receive an attacker-controlled pack before the final repository size is known. Therefore Phase 3 will not expose `git_clone` until the implementation can enforce a hard transfer/storage ceiling during object ingestion or place the clone in an OS-enforced quota boundary. Temporary clone directories must also be private, cleaned on failure, and atomically promoted only after validation.

## Validation target

Every Phase 3 slice must pass the existing Quality Gate, Security Scan, and container build workflows before remote mutations are enabled. Focused tests will cover configuration rejection, credential non-disclosure, egress blocking, redirect/proxy rejection, response limits, stale remote-head rejection, non-force push semantics, audit safety, and quota failure behavior.
