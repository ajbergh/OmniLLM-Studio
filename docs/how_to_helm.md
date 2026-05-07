# How to Deploy OmniLLM-Studio with Helm

This is the operator's guide to running OmniLLM-Studio on Kubernetes. It walks you from "I have a cluster" to "the team is using it" — including the boring parts (TLS, backups, upgrades) and the parts that catch people out (SSE buffering, the single-replica constraint, the encryption key).

If you only want the **chart's reference values and templates**, see [`deploy/helm/omnillm-studio/README.md`](../deploy/helm/omnillm-studio/README.md). If you want the **design rationale**, see [`docs/internal_docs/Kubernetes_Helm_Plan.md`](internal_docs/Kubernetes_Helm_Plan.md).

---

## Table of Contents

- [What you'll deploy](#what-youll-deploy)
- [Prerequisites](#prerequisites)
- [Tutorial: install on a local kind cluster](#tutorial-install-on-a-local-kind-cluster)
- [Production install](#production-install)
- [Topology: combined vs. split](#topology-combined-vs-split)
- [The encryption key](#the-encryption-key)
- [Storage and the single-replica constraint](#storage-and-the-single-replica-constraint)
- [TLS and Ingress](#tls-and-ingress)
- [Server-Sent Events (streaming)](#server-sent-events-streaming)
- [Upgrades](#upgrades)
- [Backups](#backups)
- [Observability](#observability)
- [Security hardening](#security-hardening)
- [Cloud-specific notes](#cloud-specific-notes)
- [Troubleshooting](#troubleshooting)
- [Uninstalling](#uninstalling)
- [FAQ](#faq)

---

## What you'll deploy

```
                  ┌─────────────── Kubernetes Pod (1 replica) ───────────────┐
  Internet ──▶    │                                                          │
       Ingress ──▶│  nginx (sidecar, port 8081)  ──proxy──▶  Go backend      │ ──▶ PVC (/data)
                  │     - serves frontend/dist           (port 8080)         │      ├── omnillm-studio.db (SQLite + WAL)
                  │     - SSE-safe /v1/* proxy                               │      ├── attachments/
                  │                                                          │      └── chromem/<conversation_id>/...
                  └──────────────────────────────────────────────────────────┘
```

The default ("combined") topology is one Pod with two containers. The "split" topology puts nginx in its own Deployment with multiple replicas and keeps the backend as the StatefulSet. Either way, the backend is **always exactly one replica** — the SQLite + chromem-go-on-local-disk design is single-writer.

The deployed Kubernetes resources are:

- 1 × `StatefulSet` (backend, optionally with the nginx sidecar)
- 1 × `Service` (user-facing)
- 1 × `Service` (headless, for StatefulSet DNS)
- 1 × `Deployment` (frontend nginx) — split topology only
- 1 × `ConfigMap` (nginx config)
- 1 × `Secret` (chart-managed, or a Secret you bring yourself)
- 1 × `Ingress`
- 1 × `PersistentVolumeClaim` (via StatefulSet `volumeClaimTemplates`)
- 1 × `ServiceAccount`
- 1 × `PodDisruptionBudget` (optional, default on)
- 1 × `NetworkPolicy` (optional, default off)

---

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| Kubernetes | 1.27+ | Earlier may work; not tested |
| Helm | 3.14+ | `brew install helm` / `choco install kubernetes-helm` |
| `kubectl` | 1.27+ | Must be authenticated against the target cluster |
| IngressController | any | Defaults assume `ingress-nginx`. HAProxy / Traefik / cloud LBs work too — see [TLS and Ingress](#tls-and-ingress). |
| StorageClass | any | Must support `ReadWriteOnce`. The default StorageClass is used unless you set `persistence.storageClass`. |
| `openssl` | any modern | To generate the master key |

You also need a clone of this repo (or a tagged release) somewhere local — the chart is sourced from `deploy/helm/omnillm-studio/` and is not yet published to a chart registry.

```bash
git clone https://github.com/ajbergh/OmniLLM-Studio.git
cd OmniLLM-Studio
```

---

## Tutorial: install on a local kind cluster

This is the recommended first run. It takes ~5 minutes and gives you a working install you can poke at.

### 1. Create a cluster with the ingress port exposed

```bash
cat <<EOF | kind create cluster --name omnillm --config -
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 8080
        protocol: TCP
      - containerPort: 443
        hostPort: 8443
        protocol: TCP
EOF
```

### 2. Install ingress-nginx

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=180s
```

### 3. Install OmniLLM-Studio

```bash
kubectl create namespace omnillm

helm install omnillm deploy/helm/omnillm-studio \
  --namespace omnillm \
  --set ingress.host=omnillm.localtest.me \
  --set secrets.encryptionKey=$(openssl rand -hex 32)
```

> `localtest.me` resolves to `127.0.0.1` automatically — no `/etc/hosts` editing needed.

### 4. Wait for the StatefulSet to come up

```bash
kubectl get pods -n omnillm -w
# Wait for omnillm-omnillm-studio-0 to show 2/2 Running
```

### 5. Smoke test

```bash
# Through the chart's own test Pod:
helm test omnillm -n omnillm

# Or directly:
curl http://omnillm.localtest.me:8080/v1/health
# {"ok":true}

# Open the app:
open http://omnillm.localtest.me:8080/   # macOS
xdg-open http://omnillm.localtest.me:8080/   # Linux
start http://omnillm.localtest.me:8080/   # Windows
```

### 6. Tear down

```bash
helm uninstall omnillm -n omnillm
kubectl delete namespace omnillm
kind delete cluster --name omnillm
```

---

## Production install

The kind tutorial above takes shortcuts that you should not take in production. Specifically:

- It puts the encryption key into Helm's release history via `--set`.
- It uses the default StorageClass without thinking.
- It serves over plain HTTP.

A production install looks more like this:

### 1. Generate and store the master key out-of-band

The `OMNILLM_MASTER_KEY` is a 32-byte AES key that encrypts every provider API key (OpenAI, Anthropic, etc.) at rest. **Lose it and you cannot decrypt the keys** — users will need to re-enter them. **Leak it and you've leaked all the API keys.**

Recommended: bring-your-own Secret managed by your secret store (sealed-secrets, external-secrets-operator, Vault, etc.). The chart reads the Secret name from `secrets.existingSecret`.

```bash
# One-time generation (do this offline, into your password manager / Vault):
openssl rand -hex 32
# 8f3c…e201

# Store it in Kubernetes via your usual secret pipeline. Example using a
# direct kubectl create (replace with your real workflow):
kubectl create namespace omnillm

kubectl create secret generic omnillm-studio-secrets \
  --namespace omnillm \
  --from-literal=OMNILLM_MASTER_KEY=8f3c…e201
```

The Secret must contain the literal key name `OMNILLM_MASTER_KEY`. Anything else and the backend won't find it.

### 2. Choose a StorageClass deliberately

Do not just rely on the default. Pick a class that:

- Supports `ReadWriteOnce` (required).
- Has snapshotting if you plan to use volume-snapshot backups.
- Is on the same zone as the nodes you're scheduling onto (or supports multi-zone via WaitForFirstConsumer binding).
- Has IOPS appropriate for SQLite. SQLite WAL is not bandwidth-heavy but is latency-sensitive; gp3 / Premium-SSD / pd-ssd are all fine. Avoid HDD-backed classes.

```bash
kubectl get storageclass
```

### 3. Author a `values.yaml`

Don't pile up `--set` flags. Put your overrides in a file and check it in (without secrets — secrets stay in `existingSecret`).

```yaml
# values.production.yaml
image:
  pullPolicy: IfNotPresent
  pullSecrets: []   # add an imagePullSecret name if you mirror to a private registry

topology: combined  # or split — see below

persistence:
  enabled: true
  storageClass: "fast-ssd"
  size: 50Gi
  retainOnDelete: true   # keeps the PVC across helm uninstall (data survives reinstall)

config:
  allowPublicRegistration: false  # solo-mode safe
  corsOrigins: []                 # nginx fronts everything; usually empty

secrets:
  existingSecret: omnillm-studio-secrets

service:
  type: ClusterIP

ingress:
  enabled: true
  className: nginx
  host: omnillm.example.com
  tls:
    enabled: true
    secretName: omnillm-tls   # the cert-manager-issued or pre-loaded TLS Secret

resources:
  backend:
    requests: { cpu: 500m, memory: 1Gi }
    limits:   { cpu: 4,    memory: 4Gi }

networkPolicy:
  enabled: true

podDisruptionBudget:
  enabled: true
  minAvailable: 1
```

### 4. Install

```bash
helm install omnillm deploy/helm/omnillm-studio \
  --namespace omnillm \
  --create-namespace \
  -f values.production.yaml
```

### 5. Verify

```bash
helm test omnillm -n omnillm
kubectl get pods,sts,svc,ingress,pvc -n omnillm
```

---

## Topology: combined vs. split

The chart ships two topology shapes. Both deploy the same single-replica backend StatefulSet. The difference is where nginx runs.

### Combined (default)

```yaml
topology: combined
```

- One Pod, two containers: backend + nginx sidecar.
- One user-facing Service.
- nginx talks to the backend on `127.0.0.1:8080` — no CORS, no service-mesh hops.
- Simplest to operate. **Pick this unless you have a reason not to.**

### Split

```yaml
topology: split
```

- Backend `StatefulSet` (1 pod).
- Frontend `Deployment` (default 2 nginx pods).
- Two Services. Ingress sends `/v1/*` to backend, everything else to frontend.

Pick split when:

- You want frontend pods on different nodes from the backend (e.g. you put the backend on a tainted "stateful" node pool).
- You want distinct NetworkPolicies on the two tiers.
- You want to scale the frontend behind a CDN.
- You have a service mesh and want each tier as its own workload.

You can switch between topologies with `helm upgrade --reuse-values --set topology=split` — but it triggers a Pod replacement, and during the swap the chart briefly has both shapes mid-rollout. Do this during a maintenance window.

---

## The encryption key

This is the part that catches people out. The backend AES-256-GCM-encrypts every provider API key it stores using a 32-byte master key (`internal/crypto/`). On Kubernetes, that key comes from the `OMNILLM_MASTER_KEY` env var.

**Three rules:**

1. **It must be exactly 64 hex characters** (32 bytes). The chart's [`values.schema.json`](../deploy/helm/omnillm-studio/values.schema.json) enforces this.
2. **The chart fails-render if you provide neither `secrets.existingSecret` nor `secrets.encryptionKey`.** This is intentional — there is no "no encryption" mode.
3. **Rotating the key invalidates all stored provider API keys.** They cannot be decrypted with the new key. Plan for users to re-enter them.

### Recommended pattern: BYO Secret

```bash
kubectl create secret generic omnillm-studio-secrets \
  --namespace omnillm \
  --from-literal=OMNILLM_MASTER_KEY=$(openssl rand -hex 32)
```

```yaml
secrets:
  existingSecret: omnillm-studio-secrets
```

The encryption key never enters Helm release history this way.

### Acceptable for dev/lab: chart-managed Secret

```yaml
secrets:
  encryptionKey: <64 hex chars>
```

The chart writes this verbatim into a `Secret`. It is also visible in `helm get values omnillm` and in the release ConfigMap kept by Helm in the cluster. Don't do this in production.

### Backing up the key

The master key belongs in your password manager / Vault / KMS, not in the cluster's etcd backup alone. If you lose etcd and the key was only in etcd, you have lost the encryption key.

---

## Storage and the single-replica constraint

The chart **fails to render** if you set `replicaCount` greater than 1:

```
omnillm-studio: replicaCount must be 1. SQLite and chromem-go on local disk
are single-writer.
```

This is enforced both by `values.schema.json` (`const: 1`) and by an explicit `fail` in `_helpers.tpl`. Why:

- **SQLite** is single-writer at the file level. Two backend pods both writing the WAL would corrupt the database.
- **chromem-go** persists to a directory on local disk. Two writers competing for the same file is undefined behavior.
- The 15-minute session-cleanup ticker is in-process and expects to be the only one running it.

You get one writer. Period. Multi-replica scale-out requires migrating to Postgres + an external vector store (Qdrant / pgvector / hosted chromem) and is documented as a future plan in [`docs/internal_docs/Kubernetes_Helm_Plan.md`](internal_docs/Kubernetes_Helm_Plan.md) §9.

### Sizing the PVC

Default is `20Gi`. Rough rule of thumb:

| Component | Per 1,000 conversations of typical use |
|---|---|
| SQLite DB (messages, metadata) | 200–500 MB |
| chromem-go vectors | 100–300 MB |
| Attachments (heavily user-dependent) | 0–multiple GB |

For a small team (~10 active users), `20Gi` is comfortable. For hundreds of users with image attachments, plan on `100Gi+`.

### What if the PVC fills up?

Most StorageClasses allow `pvc.spec.resources.requests.storage` to be edited up. Try:

```bash
kubectl edit pvc data-omnillm-omnillm-studio-0 -n omnillm
# bump .spec.resources.requests.storage, then:
kubectl rollout restart sts/omnillm-omnillm-studio -n omnillm
```

If your StorageClass doesn't support online resize, you'll need a backup-restore cycle — see [Backups](#backups).

---

## TLS and Ingress

The chart ships SSE-aware annotations for `ingress-nginx`. If you use a different controller, replicate the equivalents.

### `ingress-nginx` (default, baked in)

```yaml
ingress:
  enabled: true
  className: nginx
  host: omnillm.example.com
  tls:
    enabled: true
    secretName: omnillm-tls
```

The chart pre-applies these annotations:

```
nginx.ingress.kubernetes.io/proxy-buffering: "off"
nginx.ingress.kubernetes.io/proxy-request-buffering: "off"
nginx.ingress.kubernetes.io/proxy-read-timeout: "600"
nginx.ingress.kubernetes.io/proxy-send-timeout: "600"
nginx.ingress.kubernetes.io/proxy-http-version: "1.1"
nginx.ingress.kubernetes.io/proxy-body-size: "100m"
```

### cert-manager + Let's Encrypt

If you use cert-manager, add the issuer annotation in `ingress.annotations`:

```yaml
ingress:
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  tls:
    enabled: true
    secretName: omnillm-tls   # cert-manager populates this Secret automatically
```

### Traefik

Replace the `nginx.ingress.kubernetes.io/*` annotations with Traefik middleware. The crucial properties are: HTTP/1.1 to backends, no response buffering, idle timeouts ≥ 600 seconds.

```yaml
ingress:
  className: traefik
  annotations:
    traefik.ingress.kubernetes.io/router.middlewares: omnillm-sse-headers@kubernetescrd
```

Then deploy a `Middleware` CR that sets `responseForwarding.flushInterval: 1ms` and disables buffering.

### HAProxy

Set `timeout server 600s` and `option http-buffer-request` _off_ for the backend. The default frontend buffering in HAProxy 2.4+ does not affect SSE responses, but check with a streaming chat to be sure.

### Cloud load balancers (ALB / GCLB / Azure Application Gateway)

These often default to short idle timeouts (60 s on AWS ALB, for example). The chat will stall mid-stream. Raise the LB idle timeout to **at least 600 seconds**, and disable any "response buffering" feature the LB offers.

| LB | What to change |
|---|---|
| AWS ALB | `idle_timeout.timeout_seconds = 600` on the Load Balancer attributes |
| Google Cloud Load Balancer | Backend service `timeoutSec: 600` (or higher) |
| Azure App Gateway | `requestTimeout: 600` on the BackendHttpSetting |
| CloudFront | `OriginReadTimeout: 60` is the maximum and is **not enough** — do not put CloudFront in front of streaming routes. Use a Regional ALB. |

---

## Server-Sent Events (streaming)

Token-by-token chat, agent step events, and web search progress all flow over SSE. **Buffering anywhere in the proxy chain breaks it.** The user sees responses arrive in chunks (or not at all until completion).

The chart configures every layer it controls:

| Layer | Setting |
|---|---|
| Backend | Sends `X-Accel-Buffering: no` on every SSE response. (`message_handler.go`, `agent_handler.go`) |
| nginx (sidecar / frontend) | `proxy_buffering off`, `proxy_request_buffering off`, `chunked_transfer_encoding on`, 600 s timeouts. |
| ingress-nginx | Annotations above. |

If you change the Ingress class, **re-verify the streaming experience after each install**. It is the easiest thing to silently regress.

A 30-second smoke test:

```bash
kubectl port-forward -n omnillm svc/omnillm-omnillm-studio 8080:80
# in another terminal: open http://localhost:8080, send a chat message, watch
# tokens arrive one-at-a-time. If they arrive in a single burst at the end,
# something is buffering.
```

---

## Upgrades

### Chart-only upgrade

```bash
helm upgrade omnillm deploy/helm/omnillm-studio \
  -n omnillm \
  --reuse-values
```

### Image upgrade

```bash
helm upgrade omnillm deploy/helm/omnillm-studio \
  -n omnillm \
  --reuse-values \
  --set image.backendTag=v0.3.0 \
  --set image.frontendTag=v0.3.0
```

The StatefulSet rolls one Pod (because there is only one). The 30-second `terminationGracePeriodSeconds` plus the backend's 10-second shutdown context lets SQLite checkpoint the WAL cleanly. Schema migrations are baked into the backend's startup path (`db.Migrate`) — no separate migration job.

Plan for ~10–30 seconds of downtime on every upgrade. There is no zero-downtime path with a single writer.

### What about the Wails / desktop / web binaries?

They continue to be built by the existing `scripts/build-wails-*` and `scripts/build-web-*` scripts. Tag-pushed releases continue to publish those binaries via [`.github/workflows/release.yml`](../.github/workflows/release.yml). The container path uses a separate workflow ([`container.yml`](../.github/workflows/container.yml)) and is purely additive.

---

## Backups

The chart does not ship a backup CronJob — backup destinations are too cloud-specific. You have options.

### Online SQLite backup

SQLite's `.backup` command is safe with WAL writes in flight:

```bash
kubectl exec -n omnillm sts/omnillm-omnillm-studio -c backend -- \
  /bin/sh -c 'sqlite3 /data/omnillm-studio.db ".backup /tmp/snap.db"'
kubectl cp omnillm/omnillm-omnillm-studio-0:/tmp/snap.db ./backup-$(date +%F).db -c backend
```

This captures the SQLite database only — not attachments or chromem vectors.

### Volume snapshot (preferred if your CSI driver supports it)

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: omnillm-snap-2026-05-07
  namespace: omnillm
spec:
  volumeSnapshotClassName: csi-snapshot-class
  source:
    persistentVolumeClaimName: data-omnillm-omnillm-studio-0
```

Volume snapshots capture the full PVC: SQLite (with WAL), attachments, and chromem. They are point-in-time and atomic.

### Whole-pod tarball

```bash
kubectl exec -n omnillm sts/omnillm-omnillm-studio -c backend -- \
  tar czf - -C / data \
  > omnillm-data-$(date +%F).tgz
```

Quick and dirty. Beware: doing this against a hot SQLite DB without a checkpoint can give you an inconsistent backup. Prefer the online-backup or volume-snapshot approaches.

### Restore

1. `helm uninstall` the release (PVC retained by default).
2. Replace the PVC contents with your backup (`kubectl cp` for tarball; restore the snapshot via your CSI driver).
3. `helm install` again.

The `OMNILLM_MASTER_KEY` must be the same as when the data was encrypted, or stored provider keys will not decrypt.

---

## Observability

The chart does not yet ship Prometheus exporters or Grafana dashboards. What you have:

- **Logs:** stdout, JSON-ish (line-oriented). Use whatever log aggregator you already run (Fluent Bit → Loki / Splunk / CloudWatch).
- **Health:** `GET /v1/health` returns `{"ok":true}`. Used by liveness / readiness / startup probes.
- **Version:** `GET /v1/version` returns the build version.

Prometheus `/metrics` is on the backlog (see [`Kubernetes_Helm_Plan.md`](internal_docs/Kubernetes_Helm_Plan.md) §10 Phase 5).

Useful kubectl one-liners while debugging:

```bash
# Logs (backend container)
kubectl logs -n omnillm sts/omnillm-omnillm-studio -c backend -f

# Logs (nginx sidecar)
kubectl logs -n omnillm sts/omnillm-omnillm-studio -c frontend -f

# Resource usage
kubectl top pod -n omnillm

# Probe failures
kubectl describe pod -n omnillm omnillm-omnillm-studio-0 | grep -A 5 Probe
```

---

## Security hardening

Out-of-the-box, the chart already:

- Runs both containers as non-root (`runAsNonRoot: true`, UID 65532).
- Drops all Linux capabilities.
- Sets `readOnlyRootFilesystem: true` (writes only happen via `/data` and `/tmp` `emptyDir`).
- Disables privilege escalation.
- Uses the `RuntimeDefault` seccomp profile.

Things you should add for production:

- **Pod Security Admission**: label the namespace `pod-security.kubernetes.io/enforce=restricted`. The chart's defaults satisfy `restricted`.
  ```bash
  kubectl label namespace omnillm pod-security.kubernetes.io/enforce=restricted
  ```
- **NetworkPolicy**: enable it (`networkPolicy.enabled: true`). Default-deny ingress; the chart auto-allows the frontend → backend hop in split topology. If you also want to restrict egress (e.g. only allow LLM provider domains), author a separate NetworkPolicy — the chart does not do this for you.
- **Image provenance**: the `container.yml` workflow signs images with cosign keyless. Verify in production:
  ```bash
  cosign verify ghcr.io/ajbergh/omnillm-studio-backend:v0.2.0 \
    --certificate-identity-regexp '^https://github.com/ajbergh/OmniLLM-Studio' \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com
  ```
- **Multi-user mode**: by default `config.allowPublicRegistration: false` — the first registration creates an admin and registration is then closed. Leave this default unless you have a use case for it.
- **Rotation**: the `OMNILLM_MASTER_KEY` is not key-versioned. Rotation requires re-entering all provider keys. Pick a key once and protect it.

---

## Cloud-specific notes

### Amazon EKS

- Use `gp3` StorageClass with snapshots enabled.
- Default ALB idle timeout is 60 s — must be raised to 600 s, or use the AWS Load Balancer Controller and set `service.beta.kubernetes.io/aws-load-balancer-attributes: idle_timeout.timeout_seconds=600`.
- For ARM64 nodes (Graviton), the chart works as-is — the container images are multi-arch.

### Google GKE

- Use `pd-ssd` StorageClass.
- The default GCLB has a 30-second backend timeout — raise it on the BackendConfig.
- GKE Autopilot is fine; the chart's resource requests stay within Autopilot's limits.

### Azure AKS

- Use `managed-csi-premium` StorageClass.
- Azure Application Gateway: set `requestTimeout: 600` on the backend HTTP setting.
- Azure CNI is fine; the NetworkPolicy implementation must be `azure` or `calico` for `networkPolicy.enabled: true` to take effect.

### kind / k3d / minikube

- The kind tutorial above works as-is.
- k3d: same as kind, plus `--port "8080:80@loadbalancer"`.
- minikube: `minikube addons enable ingress` first.

---

## Troubleshooting

### `helm install` fails with "replicaCount must be 1"

You set `replicaCount` to something other than 1 in `values.yaml` or `--set`. Fix: don't. The chart is single-replica by design.

### `helm install` fails with "you must set either secrets.existingSecret or secrets.encryptionKey"

You didn't supply the master key. Pick one of:

```bash
# A: BYO Secret (recommended)
kubectl create secret generic omnillm-studio-secrets \
  -n omnillm \
  --from-literal=OMNILLM_MASTER_KEY=$(openssl rand -hex 32)
helm install … --set secrets.existingSecret=omnillm-studio-secrets

# B: chart-managed
helm install … --set secrets.encryptionKey=$(openssl rand -hex 32)
```

### `helm install` fails with "secrets.encryptionKey must be exactly 64 hex characters"

You probably ran `openssl rand -base64 32` (gives base64) instead of `openssl rand -hex 32` (gives 64 hex chars). Use the hex form.

### Pod stuck in `Pending`

```bash
kubectl describe pod -n omnillm omnillm-omnillm-studio-0
```

Most common causes:

- No StorageClass available — fix: install one or set `persistence.storageClass`.
- StorageClass has `volumeBindingMode: WaitForFirstConsumer` and the node selector / taints prevent scheduling — check `nodeSelector` / `tolerations` in your values.
- Insufficient resources — check `kubectl top nodes`.

### Pod is `Running` but `0/2` `Ready`

```bash
kubectl logs -n omnillm sts/omnillm-omnillm-studio -c backend
kubectl describe pod -n omnillm omnillm-omnillm-studio-0 | grep -A 10 Probe
```

Common causes:

- `OMNILLM_MASTER_KEY` Secret missing or wrong key name → backend logs show "get master key" errors.
- PVC mounted but not writable for UID 65532 — check the StorageClass and `fsGroup`.
- nginx waiting for the backend that hasn't passed startup probe yet — wait 30–60 seconds.

### Streaming chat stalls / arrives in one burst

Something in the proxy chain is buffering. Verify in this order:

1. `kubectl get ingress -n omnillm -o yaml | grep -A 10 annotations` — confirm the SSE annotations are present.
2. `kubectl exec -n omnillm sts/omnillm-omnillm-studio -c frontend -- cat /etc/nginx/conf.d/default.conf | grep proxy_buffering` — should show `off`.
3. Cloud LB / CDN — see [TLS and Ingress](#tls-and-ingress) above. Raise idle timeouts to 600 s. Disable response buffering.

### Provider API keys won't decrypt after a restart / reinstall

The `OMNILLM_MASTER_KEY` differs from the one used when the keys were encrypted. Recover the original key from your password manager / Vault, or have users re-enter their provider keys.

### `helm test` Pod fails

```bash
kubectl logs -n omnillm omnillm-omnillm-studio-test-connection
```

The test Pod just hits `/v1/health`. If it fails, the readiness probe is also failing — see "0/2 Ready" above.

### Out of disk space (`no space left on device`)

The PVC filled up. Either resize it (see [Storage](#storage-and-the-single-replica-constraint)) or clean up — typically attachments. There is no automated retention policy.

### `image pull failed: unauthorized`

The image registry is private and the cluster doesn't have credentials. Add an `imagePullSecret`:

```bash
kubectl create secret docker-registry ghcr-pull \
  -n omnillm \
  --docker-server=ghcr.io \
  --docker-username=<user> \
  --docker-password=<pat> \
  --docker-email=<email>
```

```yaml
image:
  pullSecrets:
    - ghcr-pull
```

---

## Uninstalling

```bash
helm uninstall omnillm -n omnillm
```

By default `persistence.retainOnDelete: true` keeps the PVC after uninstall. This means you can `helm install` again and reattach to the same data. If you want to wipe the data:

```bash
kubectl delete pvc -n omnillm data-omnillm-omnillm-studio-0
kubectl delete namespace omnillm
```

If you set `secrets.encryptionKey` directly (chart-managed Secret), the Secret is deleted with the release. If you used `secrets.existingSecret`, the Secret is yours to manage and is **not** deleted by `helm uninstall`.

---

## FAQ

**Why can't I just run more replicas?**
SQLite is single-writer at the file level, and chromem-go vector collections are per-conversation directories on local disk. Two backend pods writing to the same PVC would corrupt both. The chart's `_helpers.tpl` and `values.schema.json` enforce `replicaCount: 1` precisely because this is a recurring foot-gun. Horizontal scale requires a Postgres + external vector store migration that is not done yet.

**Can I use ReadWriteMany volumes to scale?**
No. Even with RWX, SQLite is single-writer. RWX would only help if multiple readers needed concurrent access, which is not the architecture.

**Can I bring my own database (Postgres / MySQL)?**
Not yet. The backend hard-depends on `mattn/go-sqlite3` and chromem-go. A Postgres + pgvector backend is on the roadmap (see `docs/internal_docs/Kubernetes_Helm_Plan.md` §9) and is the prerequisite for multi-replica scale.

**Is the desktop app affected by anything in this guide?**
No. The Wails desktop binary is built and distributed exactly as before. The container path is purely additive — none of the existing build scripts or release workflows are touched.

**Can I disable the encryption?**
No. The chart fails-render without a key. The encryption is mandatory and protects user-supplied API keys at rest.

**Can I run on a non-amd64/arm64 architecture?**
The published images are linux/amd64 + linux/arm64. For other architectures, build the image yourself from `deploy/docker/Dockerfile.backend` (you'll need to adapt the cross-compiler stanza).

**What's a reasonable rollout plan?**
1. Lab cluster with kind, exercise streaming and RAG.
2. Staging cluster (real cloud, real Ingress, real StorageClass), run `helm test` and exercise chat + image generation.
3. Take a baseline backup.
4. Production install with `values.production.yaml` and BYO Secret.
5. Document the disaster-recovery runbook (where the master key lives, how to restore from snapshot).

**How do I keep the chart up to date as the project evolves?**
Watch `deploy/helm/omnillm-studio/Chart.yaml` `version`. Bumps signal template-shape changes; release notes call out values that changed semantics. Image tags follow `Chart.AppVersion` by default — pin with `image.backendTag` / `image.frontendTag` if you want to control upgrades independently.

---

## Where things live

| What | Where |
|---|---|
| Helm chart | [`deploy/helm/omnillm-studio/`](../deploy/helm/omnillm-studio/) |
| Chart reference values | [`deploy/helm/omnillm-studio/README.md`](../deploy/helm/omnillm-studio/README.md) |
| Dockerfiles | [`deploy/docker/`](../deploy/docker/) |
| Local docker-compose | [`deploy/docker/docker-compose.yaml`](../deploy/docker/docker-compose.yaml) |
| CI workflow (images + helm lint) | [`.github/workflows/container.yml`](../.github/workflows/container.yml) |
| Design doc / status log | [`docs/internal_docs/Kubernetes_Helm_Plan.md`](internal_docs/Kubernetes_Helm_Plan.md) |
| Wails / web release workflow (untouched) | [`.github/workflows/release.yml`](../.github/workflows/release.yml) |
