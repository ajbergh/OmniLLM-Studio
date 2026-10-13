# Dedicated Sandbox Worker Deployment

> **Status:** implementation guide for the dedicated Linux `sandboxd` execution plane. This document does not authorize running arbitrary tenant code in the primary API container.

## Boundary

`backend/cmd/sandboxd` is the only process exposed by the worker image. It serves authenticated sandbox protocol v2 and delegates execution to the first-party `sandbox.Runtime`. The primary API talks to it through `OMNILLM_SANDBOX_URL` and `OMNILLM_SANDBOX_TOKEN`.

The worker image is deliberately separate from `Dockerfile.backend`. It contains:

- `omnillm-sandboxd` only, not `cmd/server`;
- Bubblewrap;
- a dedicated immutable Debian execution rootfs at `/opt/omnillm/rootfs`;
- Python 3 and Node inside that execution rootfs for existing Linux language shortcuts;
- a non-root worker identity (`65532:65532`);
- a writable worker-owned scratch root at `/var/lib/omnillm-sandbox`.

## Required host primitives

The image cannot safely manufacture kernel authority for itself. The deployment must provide the primitives required by the capabilities it intends to advertise.

### Bubblewrap/user namespaces

The container runtime/node must allow the non-root worker to create the user/mount/PID/network namespaces Bubblewrap requires. If unprivileged user namespaces are disabled, the worker must fail startup/runtime initialization rather than add broad Linux capabilities or run privileged merely to make Bubblewrap work.

### cgroup-v2 delegation

PID/memory quota capabilities are dynamic. To enable them, the operator must:

1. create a dedicated cgroup-v2 subtree for one worker identity;
2. delegate only the required controllers (`pids`, `memory`, and any later independently proven controller);
3. place the worker itself in a child beneath that delegated root;
4. make the configured delegation available at `OMNILLM_SANDBOX_CGROUP_ROOT` with the ownership/permissions required for per-execution child cgroups.

A missing or unusable configured cgroup root is a deployment error. An omitted root is supported, but the corresponding quota capability bits remain false and Broker admission rejects non-zero requests for those controls.

### Network

The first-party Bubblewrap runtime remains network-none. A Kubernetes/host NetworkPolicy is defense in depth; it is not a substitute for the runtime's destination allowlist capability and must not cause `network_allowlist=true` to be advertised.

Host-brokered HTTP egress, when enabled by the application, executes outside arbitrary sandbox processes and consumes explicit owner-bound network grants.

## Secrets

`OMNILLM_SANDBOX_TOKEN` is a worker authentication secret. Do not place it in an image layer, ConfigMap, command line, or model-visible environment. Inject it from the deployment secret mechanism.

The worker must not receive provider API keys, Git credentials, cloud credentials, or API-server encryption keys. Arbitrary sandbox environments reject credential/proxy-sensitive keys. Service credentials remain in trusted host-side consumers.

## Health and readiness

`GET /v2/capabilities` requires bearer authentication. Kubernetes should therefore use either:

- a TCP readiness/liveness probe on port 8090; or
- an authenticated exec probe that reads the worker token from its mounted secret without logging it.

Readiness means protocol service availability, not that every optional capability exists. Callers must still inspect the returned capabilities.

## Container invocation

Example without cgroup quotas:

```sh
docker run --rm \
  --name omnillm-sandboxd \
  --read-only \
  --tmpfs /tmp:rw,nosuid,nodev,noexec,size=64m \
  --mount type=tmpfs,destination=/var/lib/omnillm-sandbox,tmpfs-size=1073741824 \
  -e OMNILLM_SANDBOX_TOKEN \
  -p 127.0.0.1:8090:8090 \
  omnillm-sandboxd:latest
```

Whether Bubblewrap can create user namespaces from an outer Docker container is host/runtime dependent and must be validated on the target deployment. Do not compensate for a failed native prerequisite by adding `--privileged` as a product default.

## Server/Kubernetes production exit criteria

Phase 15 is complete only when the deployment chart proves all of the following on a target cluster:

- API and sandbox workers are distinct workloads/identities;
- worker ingress is restricted to the API workload on the protocol port;
- arbitrary egress from the worker pod is denied by default;
- the worker pod does not mount API persistent data, Docker socket, host root, or service-account credentials it does not require;
- non-root execution and seccomp/AppArmor/SELinux policy are explicit;
- Bubblewrap user namespaces work natively under that policy;
- cgroup delegation is intentionally configured when PID/memory capability is required;
- per-user/workspace admission occurs before work is dispatched;
- worker restarts cannot convert an interrupted side-effecting execution into an automatic replay;
- exact Helm/manifests pass validation and a native cluster smoke test.

The dedicated image closes Phase 2 packaging. It does not, by itself, close Phase 15.

## Remote worker HTTPS contract (2026-10-12)

The previous enabled Helm chart configured a non-loopback plaintext worker URL that the API's HTTPRuntime refused. The corrected chart requires HTTPS; do not loosen the API transport check.

Create two Secrets in the application namespace: one authorization Secret with OMNILLM_SANDBOX_TOKEN, and one TLS Secret containing tls.crt, tls.key and ca.crt. Issue the worker certificate with a DNS subjectAltName matching the rendered sandbox-worker Service name (not merely the ingress host). Enable with --set sandboxWorker.enabled=true, --set sandboxWorker.auth.existingSecret=sandbox-worker-auth and --set sandboxWorker.tls.existingSecret=sandbox-worker-tls.

The API connects to HTTPS, mounts only ca.crt, and preserves certificate hostname verification. The worker mounts only tls.crt and tls.key read-only, requires TLS 1.2+, and checks bearer authorization. A private OMNILLM_SANDBOX_CA_FILE is never allowed to enable plaintext HTTP. Non-cluster loopback workers may still use explicit HTTP.

CI exercises real non-root read-only worker container TLS capabilities, wrong/missing bearer rejection and session creation; Helm assurance checks enabled Secret wiring and negative renders. This is NOT proof of full Kubernetes nested user namespaces, delegated cgroups, arbitrary code execution, certificate rotation or crash/restart across real cluster nodes. Those acceptance gates remain open under #338.
