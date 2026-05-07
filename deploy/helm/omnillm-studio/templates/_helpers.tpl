{{/*
Common helpers for the omnillm-studio chart.
*/}}

{{/*
Replica guard — SQLite + chromem-go are single-writer.
Render this somewhere that always evaluates (every workload template includes it).
*/}}
{{- define "omnillm-studio.checkReplicas" -}}
{{- if gt (int .Values.replicaCount) 1 -}}
{{- fail "omnillm-studio: replicaCount must be 1. SQLite and chromem-go on local disk are single-writer; horizontal scale requires the Postgres + external-vector-store path documented in docs/internal_docs/Kubernetes_Helm_Plan.md §9." -}}
{{- end -}}
{{- end -}}

{{/*
Encryption-key guard — fail-fast if neither an existing Secret nor an inline key was provided.
*/}}
{{- define "omnillm-studio.checkSecrets" -}}
{{- if and (not .Values.secrets.existingSecret) (not .Values.secrets.encryptionKey) -}}
{{- fail "omnillm-studio: you must set either secrets.existingSecret (recommended) or secrets.encryptionKey (64 hex chars / 32 bytes). Generate one with: openssl rand -hex 32" -}}
{{- end -}}
{{- if and (not .Values.secrets.existingSecret) .Values.secrets.encryptionKey -}}
{{- if ne (len .Values.secrets.encryptionKey) 64 -}}
{{- fail (printf "omnillm-studio: secrets.encryptionKey must be exactly 64 hex characters (32 bytes); got %d" (len .Values.secrets.encryptionKey)) -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Topology guard.
*/}}
{{- define "omnillm-studio.checkTopology" -}}
{{- if not (or (eq .Values.topology "combined") (eq .Values.topology "split")) -}}
{{- fail (printf "omnillm-studio: topology must be 'combined' or 'split'; got %q" .Values.topology) -}}
{{- end -}}
{{- end -}}

{{/*
Aggregate validation — call once from each top-level template.
*/}}
{{- define "omnillm-studio.validate" -}}
{{- include "omnillm-studio.checkReplicas" . -}}
{{- include "omnillm-studio.checkSecrets" . -}}
{{- include "omnillm-studio.checkTopology" . -}}
{{- end -}}

{{/*
Expand the name of the chart.
*/}}
{{- define "omnillm-studio.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "omnillm-studio.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Resource names per topology.
*/}}
{{- define "omnillm-studio.backendName" -}}
{{- if eq .Values.topology "split" -}}
{{- printf "%s-backend" (include "omnillm-studio.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- include "omnillm-studio.fullname" . -}}
{{- end -}}
{{- end -}}

{{- define "omnillm-studio.frontendName" -}}
{{- printf "%s-frontend" (include "omnillm-studio.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "omnillm-studio.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "omnillm-studio.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: omnillm-studio
{{- end -}}

{{/*
Selector labels (must be stable across upgrades).
*/}}
{{- define "omnillm-studio.selectorLabels" -}}
app.kubernetes.io/name: {{ include "omnillm-studio.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "omnillm-studio.backendSelectorLabels" -}}
{{ include "omnillm-studio.selectorLabels" . }}
app.kubernetes.io/component: backend
{{- end -}}

{{- define "omnillm-studio.frontendSelectorLabels" -}}
{{ include "omnillm-studio.selectorLabels" . }}
app.kubernetes.io/component: frontend
{{- end -}}

{{/*
ServiceAccount name.
*/}}
{{- define "omnillm-studio.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "omnillm-studio.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Image references.
*/}}
{{- define "omnillm-studio.backendImage" -}}
{{- $tag := default .Chart.AppVersion .Values.image.backendTag -}}
{{- printf "%s/%s-backend:%s" .Values.image.registry .Values.image.repository $tag -}}
{{- end -}}

{{- define "omnillm-studio.frontendImage" -}}
{{- $tag := default .Chart.AppVersion .Values.image.frontendTag -}}
{{- printf "%s/%s-frontend:%s" .Values.image.registry .Values.image.repository $tag -}}
{{- end -}}

{{/*
Secret name to reference (existing or chart-managed).
*/}}
{{- define "omnillm-studio.secretName" -}}
{{- if .Values.secrets.existingSecret -}}
{{- .Values.secrets.existingSecret -}}
{{- else -}}
{{- printf "%s-secrets" (include "omnillm-studio.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
ConfigMap name for nginx config.
*/}}
{{- define "omnillm-studio.nginxConfigMapName" -}}
{{- printf "%s-nginx" (include "omnillm-studio.fullname" .) -}}
{{- end -}}
