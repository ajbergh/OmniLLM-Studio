# omnillm-studio Helm chart

Kubernetes deployment of [OmniLLM-Studio](https://github.com/ajbergh/OmniLLM-Studio) — a local-first LLM chat application with a Go backend and a React/TypeScript frontend.

This chart is **additive** to the project's existing distribution channels. The Wails desktop binaries and the headless web binaries continue to be built by the existing `scripts/build-wails-*` and `scripts/build-web-*` scripts, unchanged.

---

## Prerequisites

- Kubernetes 1.27+
- Helm 3.14+
- An IngressController (defaults assume `ingress-nginx`)
- A `StorageClass` that supports `ReadWriteOnce` PVCs

## Single-replica architecture

The backend stores state in three places on local disk:

- **SQLite** (`omnillm-studio.db` + WAL) — single-writer
- **Attachments** (`/data/attachments/`) — uploaded files
- **chromem-go** (`/data/chromem/<conversation_id>/`) — per-conversation vector collections

Because of this, the chart deploys a **StatefulSet with `replicas: 1`** backed by a single PVC. The chart will fail to render if you set `replicaCount` higher than 1.

Horizontal scale-out requires migrating to Postgres + an external vector store and is tracked in [`docs/internal_docs/Kubernetes_Helm_Plan.md`](../../../docs/internal_docs/Kubernetes_Helm_Plan.md) §9.

## Topologies

| Topology | What it deploys | When to use |
|---|---|---|
| `combined` (default) | One Pod with two containers: Go backend + nginx sidecar. One Service. | Most installs. Simplest, lowest latency. |
| `split` | Backend StatefulSet (1 replica) + Frontend Deployment (2 replicas). Two Services. | When you want to scale frontend, apply distinct NetworkPolicies, or front it with a CDN. |

## Install

### 1. Generate a master encryption key

The backend uses AES-256-GCM (`internal/crypto/`) to encrypt provider API keys at rest. The chart will fail-render if you don't supply this key.

```bash
openssl rand -hex 32
# e.g. 8f3c...e201
```

### 2. Create the namespace

```bash
kubectl create namespace omnillm
```

### 3. Install

**Recommended: bring-your-own Secret** (the encryption key never enters Helm's release history):

```bash
kubectl create secret generic omnillm-studio-secrets \
  --namespace omnillm \
  --from-literal=OMNILLM_MASTER_KEY=$(openssl rand -hex 32)

helm install omnillm deploy/helm/omnillm-studio \
  --namespace omnillm \
  --set ingress.host=omnillm.example.com \
  --set secrets.existingSecret=omnillm-studio-secrets
```

**Quick-start: chart-managed Secret** (fine for dev / kind):

```bash
helm install omnillm deploy/helm/omnillm-studio \
  --namespace omnillm \
  --set ingress.host=omnillm.local \
  --set secrets.encryptionKey=$(openssl rand -hex 32)
```

### 4. Test the install

```bash
helm test omnillm --namespace omnillm
```

The test Pod hits `/v1/health` through the chart's Service.

## Common values

| Key | Default | Purpose |
|---|---|---|
| `topology` | `combined` | `combined` or `split` |
| `image.registry` | `ghcr.io` | Image registry |
| `image.repository` | `ajbergh/omnillm-studio` | Final repos: `<reg>/<repo>-backend` and `-frontend` |
| `image.backendTag` / `frontendTag` | `""` (= `Chart.AppVersion`) | Pin a specific tag |
| `persistence.size` | `20Gi` | PVC size |
| `persistence.storageClass` | `""` (cluster default) | StorageClass name |
| `ingress.host` | `omnillm.local` | Public hostname |
| `ingress.tls.enabled` / `secretName` | `false` / `""` | TLS termination at Ingress |
| `config.allowPublicRegistration` | `false` | First registrant is admin in solo mode |
| `config.corsOrigins` | `[]` | Extra origins for the API (rarely needed; nginx fronts everything) |
| `secrets.existingSecret` | `""` | BYO Secret containing `OMNILLM_MASTER_KEY` |
| `secrets.encryptionKey` | `""` | Inline 64-hex-char key (chart-managed Secret) |
| `networkPolicy.enabled` | `false` | Restrict Pod ingress |
| `podDisruptionBudget.enabled` | `true` | Ensure at least 1 Pod stays up during voluntary disruption |

See [`values.yaml`](values.yaml) for the full list. [`values.schema.json`](values.schema.json) enforces the required invariants (`replicaCount == 1`, encryption-key format, etc.).

## Streaming / SSE

Token-by-token chat, agent step events, and web search progress all use Server-Sent Events. Buffering anywhere in the proxy chain breaks the experience.

The chart sets:

- nginx (sidecar / frontend container): `proxy_buffering off`, `proxy_request_buffering off`, `chunked_transfer_encoding on`, 600 s read/send timeouts.
- ingress-nginx annotations: `proxy-buffering: off`, `proxy-read-timeout: 600`, `proxy-send-timeout: 600`, `proxy-http-version: 1.1`.
- Backend: emits `X-Accel-Buffering: no` on every SSE response.

If you swap in HAProxy / Traefik / a cloud LB, replicate those properties — most importantly disabling response buffering and lifting idle timeouts to ≥ 600 s.

## Backups

The chart does not ship a backup CronJob. Recommended approaches:

```bash
# 1. Online SQLite backup via the live container (safe with WAL):
kubectl exec -n omnillm sts/omnillm-backend -c backend -- \
  /bin/sh -c 'sqlite3 /data/omnillm-studio.db ".backup /tmp/snap.db"' \
  && kubectl cp omnillm/omnillm-backend-0:/tmp/snap.db ./snap.db -c backend
```

```bash
# 2. Volume-snapshot the PVC if your CSI driver supports it:
kubectl get pvc -n omnillm
# … VolumeSnapshot manifest pointing at the data-omnillm-backend-0 PVC
```

## Upgrade

```bash
helm upgrade omnillm deploy/helm/omnillm-studio \
  --namespace omnillm \
  --reuse-values \
  --set image.backendTag=<new-tag> \
  --set image.frontendTag=<new-tag>
```

Schema migrations run automatically on Pod start (`db.Migrate`). The PVC is retained across upgrades. The 30 s `terminationGracePeriodSeconds` plus the backend's 10 s shutdown context lets SQLite checkpoint the WAL cleanly on rollouts.

## Uninstall

```bash
helm uninstall omnillm --namespace omnillm
```

By default the chart sets `helm.sh/resource-policy: keep` on the data PVC (`persistence.retainOnDelete: true`) so you can reinstall and reattach the same data. Delete the PVC explicitly when you're sure:

```bash
kubectl delete pvc -n omnillm data-omnillm-backend-0
```

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Chart render fails: *"replicaCount must be 1"* | You set `replicaCount > 1`. SQLite + chromem-go are single-writer. |
| Chart render fails: *"you must set either secrets.existingSecret or secrets.encryptionKey"* | Provide one or the other. |
| `helm test` Pod fails health check | Check `kubectl logs sts/<name>-backend -c backend` — usually a missing env var or PVC permissions. |
| Streaming chats stall / arrive in bursts | Some proxy in the chain is buffering. Verify the Ingress annotations rendered, and (if using a cloud LB) raise its idle timeout above 600 s. |
| Provider API keys won't decrypt after restart | The `OMNILLM_MASTER_KEY` differs from the one in use when keys were encrypted. Recover the original key, or re-enter provider keys in the UI. |

## Local smoke testing

The chart developer's smoke harness lives at [`deploy/docker/docker-compose.yaml`](../../docker/docker-compose.yaml) and exercises the same images the chart ships:

```bash
docker compose -f deploy/docker/docker-compose.yaml up --build
open http://localhost:8081
```

## Status

This chart is in **active development**. See [`docs/internal_docs/Kubernetes_Helm_Plan.md`](../../../docs/internal_docs/Kubernetes_Helm_Plan.md) for the design, the backend prerequisites, and the phased rollout.
