{{- define "lago-alpha.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "lago-alpha.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "lago-alpha.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "lago-alpha.labels" -}}
app.kubernetes.io/name: {{ include "lago-alpha.name" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "lago-alpha.selectorLabels" -}}
app.kubernetes.io/name: {{ include "lago-alpha.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "lago-alpha.serviceAccountName.api" -}}
{{- if .Values.serviceAccounts.api.create -}}
{{- default (printf "%s-api" (include "lago-alpha.fullname" .)) .Values.serviceAccounts.api.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccounts.api.name -}}
{{- end -}}
{{- end -}}

{{- define "lago-alpha.serviceAccountName.replayWorker" -}}
{{- if .Values.serviceAccounts.replayWorker.create -}}
{{- default (printf "%s-replay-worker" (include "lago-alpha.fullname" .)) .Values.serviceAccounts.replayWorker.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccounts.replayWorker.name -}}
{{- end -}}
{{- end -}}

{{- define "lago-alpha.serviceAccountName.replayDispatcher" -}}
{{- if .Values.serviceAccounts.replayDispatcher.create -}}
{{- default (printf "%s-replay-dispatcher" (include "lago-alpha.fullname" .)) .Values.serviceAccounts.replayDispatcher.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccounts.replayDispatcher.name -}}
{{- end -}}
{{- end -}}

{{- define "lago-alpha.serviceAccountName.web" -}}
{{- if .Values.serviceAccounts.web.create -}}
{{- default (printf "%s-web" (include "lago-alpha.fullname" .)) .Values.serviceAccounts.web.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccounts.web.name -}}
{{- end -}}
{{- end -}}
