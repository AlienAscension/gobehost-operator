# Helm Chart + Docker Image Build/Push Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Configure the build to push a container image to `linusdb/gobehost` on Docker Hub using podman, and create a manual Helm chart for installing the operator.

**Architecture:** Update Makefile defaults for image repo and container tool. Create a Helm chart under `charts/gobehost-operator/` with templates for CRDs, deployment, RBAC, webhooks, and service. Generate a flat `dist/install.yaml` for non-Helm users.

**Tech Stack:** podman, Helm 3, kustomize, make

---

### Task 1: Update Makefile for podman and image repo

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Update IMG default and CONTAINER_TOOL**

In `Makefile`, change:
```
IMG ?= controller:latest
```
to:
```
IMG ?= linusdb/gobehost:latest
```

And change:
```
CONTAINER_TOOL ?= docker
```
to:
```
CONTAINER_TOOL ?= podman
```

- [ ] **Step 2: Add version variable and-tag targets**

Add near the top of Makefile (after IMG line):
```makefile
VERSION ?= v0.1.0
```

Add these targets after `docker-push`:
```makefile
.PHONY: docker-tag
docker-tag: ## Tag the image with a version.
	$(CONTAINER_TOOL) tag ${IMG} linusdb/gobehost:${VERSION}

.PHONY: docker-push-version
docker-push-version: docker-tag docker-push ## Push image with version tag.
	$(CONTAINER_TOOL) push linusdb/gobehost:${VERSION}
```

- [ ] **Step 3: Verify make build still works**

```bash
make build
```

Expected: `bin/manager` binary created successfully.

- [ ] **Step 4: Commit**

```bash
git add Makefile && git commit -m "feat: configure podman and linusdb/gobehost image repo"
```

---

### Task 2: Build and push the container image

**Files:** None (just build and push)

- [ ] **Step 1: Build the image**

```bash
make docker-build IMG=linusdb/gobehost:v0.1.0
```

- [ ] **Step 2: Push the image**

```bash
make docker-push IMG=linusdb/gobehost:v0.1.0
```

- [ ] **Step 3: Also tag and push latest**

```bash
podman tag linusdb/gobehost:v0.1.0 linusdb/gobehost:latest
podman push linusdb/gobehost:latest
```

- [ ] **Step 4: Verify the image exists on Docker Hub**

```bash
podman inspect docker.io/linusdb/gobehost:v0.1.0 | head -5
```

---

### Task 3: Create Helm chart structure

**Files:**
- Create: `charts/gobehost-operator/Chart.yaml`
- Create: `charts/gobehost-operator/values.yaml`
- Create: `charts/gobehost-operator/templates/_helpers.tpl`
- Create: `charts/gobehost-operator/templates/deployment.yaml`
- Create: `charts/gobehost-operator/templates/service.yaml`
- Create: `charts/gobehost-operator/templates/serviceaccount.yaml`
- Create: `charts/gobehost-operator/templates/clusterrole-manager.yaml`
- Create: `charts/gobehost-operator/templates/clusterrolebinding-manager.yaml`
- Create: `charts/gobehost-operator/templates/webhook-service.yaml`
- Create: `charts/gobehost-operator/templates/webhooks.yaml`
- Create: `charts/gobehost-operator/crds/games.gobehost.com_gameservers.yaml`

- [ ] **Step 1: Create chart directory structure**

```bash
mkdir -p charts/gobehost-operator/templates
mkdir -p charts/gobehost-operator/crds
```

- [ ] **Step 2: Create Chart.yaml**

Create `charts/gobehost-operator/Chart.yaml`:
```yaml
apiVersion: v2
name: gobehost-operator
description: A Kubernetes operator for managing game servers
type: application
version: 0.1.0
appVersion: "0.1.0"
maintainers:
  - name: gobehost
keywords:
  - kubernetes
  - operator
  - game-server
  - minecraft
home: https://github.com/gobehost/gobehost-operator
```

- [ ] **Step 3: Create values.yaml**

Create `charts/gobehost-operator/values.yaml`:
```yaml
replicaCount: 1

image:
  repository: linusdb/gobehost
  pullPolicy: IfNotPresent
  tag: "v0.1.0"

nameOverride: ""
fullnameOverride: ""

serviceAccount:
  create: true
  annotations: {}
  name: ""

resources:
  limits:
    cpu: 500m
    memory: 128Mi
  requests:
    cpu: 10m
    memory: 64Mi

nodeSelector: {}

tolerations: []

affinity: {}

manager:
  metrics:
    bindAddress: "0"
  healthProbe:
    bindAddress: ":8081"
  webhook:
    port: 9443

certmanager:
  enabled: true
```

- [ ] **Step 4: Create _helpers.tpl**

Create `charts/gobehost-operator/templates/_helpers.tpl`:
```yaml
{{- define "gobehost-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "gobehost-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name }}
{{- end }}
{{- end }}
{{- end }}

{{- define "gobehost-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "gobehost-operator.labels" -}}
helm.sh/chart: {{ include "gobehost-operator.chart" . }}
{{ include "gobehost-operator.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "gobehost-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gobehost-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "gebhost-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "gobehost-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
```

- [ ] **Step 5: Create deployment.yaml**

Create `charts/gobehost-operator/templates/deployment.yaml`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "gobehost-operator.fullname" . }}-controller-manager
  labels:
    {{- include "gobehost-operator.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      {{- include "gobehost-operator.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      annotations:
        kubectl.kubernetes.io/default-container: manager
      labels:
        {{- include "gobehost-operator.selectorLabels" . | nindent 8 }}
    spec:
      containers:
        - name: manager
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          args:
            - --leader-elect
          {{- if .Values.manager.metrics.bindAddress }}
            - --metrics-bind-address={{ .Values.manager.metrics.bindAddress }}
          {{- end }}
            - --health-probe-bind-address={{ .Values.manager.healthProbe.bindAddress }}
            - --webhook-port={{ .Values.manager.webhook.port }}
          env:
            - name: ENABLE_WEBHOOKS
              value: "true"
          ports:
            - containerPort: {{ .Values.manager.webhook.port }}
              name: webhook-server
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8081
            initialDelaySeconds: 15
            periodSeconds: 20
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8081
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
          volumeMounts:
            - mountPath: /tmp/k8s-webhook-server/serving-certs
              name: cert
              readOnly: true
      securityContext:
        runAsNonRoot: true
      serviceAccountName: {{ include "gobehost-operator.serviceAccountName" . }}
      terminationGracePeriodSeconds: 10
      volumes:
        - name: cert
          secret:
            defaultMode: 420
            secretName: webhook-server-cert
```

- [ ] **Step 6: Create service.yaml (webhook + metrics)**

Create `charts/gobehost-operator/templates/service.yaml`:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ include "gobehost-operator.fullname" . }}-webhook-service
  labels:
    {{- include "gobehost-operator.labels" . | nindent 4 }}
spec:
  ports:
    - port: 443
      protocol: TCP
      targetPort: {{ .Values.manager.webhook.port }}
  selector:
    {{- include "gobehost-operator.selectorLabels" . | nindent 4 }}
```

- [ ] **Step 7: Create serviceaccount.yaml**

Create `charts/gobehost-operator/templates/serviceaccount.yaml`:
```yaml
{{- if .Values.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "gobehost-operator.serviceAccountName" . }}
  labels:
    {{- include "gobehost-operator.labels" . | nindent 4 }}
  {{- with .Values.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- end }}
```

- [ ] **Step 8: Create clusterrole-manager.yaml**

Create `charts/gobehost-operator/templates/clusterrole-manager.yaml`:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ include "gobehost-operator.fullname" . }}-manager-role
  labels:
    {{- include "gobehost-operator.labels" . | nindent 4 }}
rules:
  - apiGroups:
      - games.gobehost.com
    resources:
      - gameservers
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
  - apiGroups:
      - games.gobehost.com
    resources:
      - gameservers/finalizers
    verbs:
      - update
  - apiGroups:
      - games.gobehost.com
    resources:
      - gameservers/status
    verbs:
      - get
      - update
      - patch
  - apiGroups:
      - apps
    resources:
      - statefulsets
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
  - apiGroups:
      - ""
    resources:
      - services
      - persistentvolumeclaims
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
  - apiGroups:
      - events.k8s.io
    resources:
      - events
    verbs:
      - create
      - patch
```

- [ ] **Step 9: Create clusterrolebinding-manager.yaml**

Create `charts/gobehost-operator/templates/clusterrolebinding-manager.yaml`:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "gobehost-operator.fullname" . }}-manager-rolebinding
  labels:
    {{- include "gobehost-operator.labels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ include "gobehost-operator.fullname" . }}-manager-role
subjects:
  - kind: ServiceAccount
    name: {{ include "gobehost-operator.serviceAccountName" . }}
    namespace: {{ .Release.Namespace }}
```

- [ ] **Step 10: Create webhooks.yaml**

Create `charts/gobehost-operator/templates/webhooks.yaml`:
```yaml
{{- if .Values.certmanager.enabled }}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: {{ include "gobehost-operator.fullname" . }}-serving-cert
  namespace: {{ .Release.Namespace }}
spec:
  dnsNames:
    - {{ include "gobehost-operator.fullname" . }}-webhook-service.{{ .Release.Namespace }}.svc
    - {{ include "gobehost-operator.fullname" . }}-webhook-service.{{ .Release.Namespace }}.svc.cluster.local
  issuerRef:
    kind: Issuer
    name: {{ include "gobehost-operator.fullname" . }}-selfsigned-issuer
  secretName: webhook-server-cert
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: {{ include "gobehost-operator.fullname" . }}-selfsigned-issuer
  namespace: {{ .Release.Namespace }}
spec:
  selfSigned: {}
{{- end }}
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: {{ include "gobehost-operator.fullname" . }}-mutating-webhook-configuration
  annotations:
    cert-manager.io/inject-ca-from: {{ .Release.Namespace }}/{{ include "gobehost-operator.fullname" . }}-serving-cert
webhooks:
  - admissionReviewVersions:
      - v1
    clientConfig:
      service:
        name: {{ include "gobehost-operator.fullname" . }}-webhook-service
        namespace: {{ .Release.Namespace }}
        path: /mutate-games-gobehost-com-v1alpha1-gameserver
    failurePolicy: Fail
    name: mgameserver-v1alpha1.kb.io
    rules:
      - apiGroups:
          - games.gobehost.com
        apiVersions:
          - v1alpha1
        operations:
          - CREATE
          - UPDATE
        resources:
          - gameservers
    sideEffects: None
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: {{ include "gobehost-operator.fullname" . }}-validating-webhook-configuration
  annotations:
    cert-manager.io/inject-ca-from: {{ .Release.Namespace }}/{{ include "gobehost-operator.fullname" . }}-serving-cert
webhooks:
  - admissionReviewVersions:
      - v1
    clientConfig:
      service:
        name: {{ include "gobehost-operator.fullname" . }}-webhook-service
        namespace: {{ .Release.Namespace }}
        path: /validate-games-gobehost-com-v1alpha1-gameserver
    failurePolicy: Fail
    name: vgameserver-v1alpha1.kb.io
    rules:
      - apiGroups:
          - games.gobehost.com
        apiVersions:
          - v1alpha1
        operations:
          - CREATE
          - UPDATE
        resources:
          - gameservers
    sideEffects: None
```

- [ ] **Step 11: Copy CRD into the chart**

```bash
cp config/crd/bases/games.gobehost.com_gameservers.yaml charts/gobehost-operator/crds/
```

- [ ] **Step 12: Validate the chart**

```bash
helm lint charts/gobehost-operator/
```

If helm is not installed:
```bash
sudo pacman -S --noconfirm helm || curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

Then run lint again.

- [ ] **Step 13: Template the chart to verify**

```bash
helm template gobehost-operator charts/gobehost-operator/ --namespace gobehost-operator-system | head -50
```

Verify the output contains Deployment, Service, ServiceAccount, ClusterRole, ClusterRoleBinding, Certificate, MutatingWebhookConfiguration, ValidatingWebhookConfiguration.

- [ ] **Step 14: Commit the Helm chart**

```bash
git add charts/ && git commit -m "feat: add Helm chart for gobehost-operator

- Manual Helm chart with full operator deployment
- CRDs as chart crds/ directory
- Webhook configuration with cert-manager support
- Configurable image, replicas, resources
- RBAC for GameServer, StatefulSet, Service, PVC, Events"
```

---

### Task 4: Generate dist/install.yaml for non-Helm install

**Files:**
- Create: `dist/install.yaml`

- [ ] **Step 1: Generate consolidated install manifest**

```bash
make build-installer IMG=linusdb/gobehost:v0.1.0
```

Verify `dist/install.yaml` was created and contains CRDs, RBAC, Deployment, Webhooks.

- [ ] **Step 2: Verify install.yaml contents**

```bash
grep -c "^---" dist/install.yaml
head -5 dist/install.yaml
```

Should have multiple YAML documents.

- [ ] **Step 3: Commit**

```bash
git add dist/ && git commit -m "feat: generate dist/install.yaml for non-Helm installation

Kustomize-built manifest with CRDs, RBAC, Deployment, and Webhooks
Image: linusdb/gobehost:v0.1.0"
```

---

### Task 5: Add Helm chart packaging targets to Makefile

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add Helm targets**

Add to Makefile (in the Deployment section):
```makefile
.PHONY: helm-package
helm-package: ## Package the Helm chart into a .tgz
	helm package charts/gobehost-operator/ -d dist/

.PHONY: helm-lint
helm-lint: ## Lint the Helm chart
	helm lint charts/gobehost-operator/

.PHONY: helm-template
helm-template: ## Template the Helm chart (dry-run)
	helm template gobehost-operator charts/gobehost-operator/ --namespace gobehost-operator-system
```

- [ ] **Step 2: Test the new targets**

```bash
make helm-lint
make helm-package
ls -la dist/*.tgz
```

- [ ] **Step 3: Commit**

```bash
git add Makefile && git commit -m "feat: add Helm lint, package, and template targets to Makefile"
```

---

## Self-Review Checklist

- **Spec coverage:** Image build/push (Task 2), Helm chart (Task 3), install.yaml (Task 4), Makefile targets (Tasks 1+5) all covered.
- **Placeholder scan:** No TBDs, all steps have concrete commands or code.
- **Type consistency:** Helm templates reference values consistently (image.repository, image.tag, etc.)
- **Gap check:** Cert-manager webhook secret is generated via cert-manager (requires cert-manager installed). For non-cert-manager installs, users would need to generate certs manually. This is documented via the `certmanager.enabled` values flag.