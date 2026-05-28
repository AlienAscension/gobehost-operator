# GobeHost Operator — Phase 1 Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold the Kubernetes Operator project with Kubebuilder, define the core GameServer CRD, and implement its controller with StatefulSet/PVC/Service reconciliation, status conditions, and finalizers.

**Architecture:** Operator built with Kubebuilder v4 + controller-runtime in Go. The GameServer CRD is the primary API. The controller reconciles a GameServer into a StatefulSet, PersistentVolumeClaim, and Service. Finalizers ensure graceful shutdown. Status conditions expose health. The design uses a game adapter pattern so game-specific logic stays out of the generic controller.

**Tech Stack:** Go 1.26, Kubebuilder v4.13.1, controller-runtime, kustomize, envtest, ginkgo/gomega

---

## File Structure

```
gobehost-operator/
├── api/v1alpha1/
│   ├── gameserver_types.go
│   ├── gameserver_webhook.go
│   ├── gameserver_webhook_test.go
│   ├── gameserver_test.go
│   └── groupversion_info.go
├── internal/
│   ├── adapter/
│   │   ├── adapter.go
│   │   ├── registry.go
│   │   ├── minecraft.go
│   │   └── adapter_test.go
│   ├── controller/
│   │   └── gameserver_controller.go
│   └── reconciler/
│       ├── pvc.go
│       ├── statefulset.go
│       ├── service.go
│       ├── status.go
│       ├── finalizer.go
│       ├── labels.go
│       └── builder_test.go
├── config/
│   ├── crd/
│   ├── default/
│   ├── manager/
│   ├── rbac/
│   ├── samples/
│   └── webhook/
├── cmd/
│   └── main.go
├── Dockerfile
├── Makefile
├── go.mod
├── PROJECT
└── plan.md
```

---

### Task 0: Environment Setup

- [ ] **Step 1: Install Go**

```bash
sudo pacman -S --noconfirm go
```

Verify: `go version` outputs go1.26.x

- [ ] **Step 2: Add GOPATH/bin to PATH**

```bash
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.zshrc && source ~/.zshrc
```

- [ ] **Step 3: Install kubebuilder**

```bash
yay -S --noconfirm kubebuilder-bin
```

Verify: `kubebuilder version`

- [ ] **Step 4: Install controller-gen and setup-envtest**

```bash
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
```

Verify: `controller-gen --version`

---

### Task 1: Kubebuilder Project Init

- [ ] **Step 1: Initialize the project**

```bash
cd /home/linus/git/gobehost-operator
kubebuilder init --domain gobehost.com --repo github.com/gobehost/operator --skip-go-version-check
```

- [ ] **Step 2: Verify build**

```bash
make build
```

Expected: `bin/manager` binary created.

- [ ] **Step 3: Verify manifests**

```bash
make manifests
```

- [ ] **Step 4: Commit the scaffold**

```bash
git add -A
git commit -m "feat: initialize kubebuilder project scaffold

Domain: gobehost.com
Repo: github.com/gobehost/operator"
```

---

### Task 2: Create the GameServer CRD API Types

- [ ] **Step 1: Scaffold the GameServer API + controller**

```bash
kubebuilder create api --group games --version v1alpha1 --kind GameServer --resource --controller
kubebuilder create webhook --group games --version v1alpha1 --kind GameServer --defaulting --programmatic
```

- [ ] **Step 2: Write GameServerSpec and GameServerStatus types**

Replace `api/v1alpha1/gameserver_types.go` with the full spec/status types (GameSpec, RuntimeSpec, StorageSpec, NetworkSpec, PortSpec, ServerSpec, BackupSpec, SchedulingSpec, SecuritySpec, GameServerSpec, GameServerStatus, GameServerPhase constants, PortInfo, GameServerFinalizer constant). Include all `+kubebuilder` markers for printcolumns, shortnames, and subresource:status.

Key types to define:
- `GameServerSpec` with fields: Game, Runtime, Resources, Storage, Network, Server, Backup, Scheduling, Security
- `GameServerStatus` with fields: Conditions, Phase, Ready, Endpoint, Address, Ports, PlayerCount, ObservedGeneration
- `GameServerPhase` constants: Pending, Provisioning, Running, Stopping, Stopped, Failed, BackupInProgress
- `GameServerFinalizer = "games.gobehost.com/finalizer"`

- [ ] **Step 3: Write type unit tests**

Create `api/v1alpha1/gameserver_test.go` — test that GameServer fields set correctly, finalizer constant exists, phase constants match.

- [ ] **Step 4: Run tests**

```bash
go test ./api/v1alpha1/... -v
```

- [ ] **Step 5: Generate CRD manifests**

```bash
make manifests
```

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "feat: define GameServer CRD types with spec, status, phases, and markers"
```

---

### Task 3: Implement Defaulting and Validation Webhooks

- [ ] **Step 1: Implement Default() on GameServer**

In `api/v1alpha1/gameserver_webhook.go`:

Default:
- `ImagePullPolicy` → `IfNotPresent`
- `Storage.AccessModes` → `[ReadWriteOnce]`
- `Network.ServiceType` → `LoadBalancer`
- `Port.Protocol` → `TCP`
- `Port.TargetPort` → same as `Port`
- `Security.SeccompProfile` → `RuntimeDefault`

- [ ] **Step 2: Implement ValidateCreate/ValidateUpdate/ValidateDelete**

Validation rules:
- `game.type` required
- `game.version` required
- `runtime.image` required
- `network.ports` must have at least one entry
- `storage.size` must not be empty
- Port values 1-65535
- `game.type` is immutable on update

- [ ] **Step 3: Write webhook tests**

Create `api/v1alpha1/gameserver_webhook_test.go` — test defaulting rules, validation rejection for required fields, immutability of game.type.

- [ ] **Step 4: Run tests**

```bash
go test ./api/v1alpha1/... -v
```

- [ ] **Step 5: Generate manifests**

```bash
make manifests
```

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "feat: implement GameServer defaulting and validation webhooks"
```

---

### Task 4: Implement Game Adapter Interface and Registry

- [ ] **Step 1: Create adapter interface**

Create `internal/adapter/adapter.go`:

```go
type GameAdapter interface {
    Name() string
    Env(gs *gamesv1alpha1.GameServer) []corev1.EnvVar
    Command(gs *gamesv1alpha1.GameServer) []string
    Args(gs *gamesv1alpha1.GameServer) []string
    Probes(gs *gamesv1alpha1.GameServer) (*corev1.Probe, *corev1.Probe)
    DataPath(gs *gamesv1alpha1.GameServer) string
}
```

- [ ] **Step 2: Create adapter registry**

Create `internal/adapter/registry.go` with `Register()`, `Get()`, `MustGet()`, `KnownGames()` functions using a `map[string]GameAdapter`.

- [ ] **Step 3: Implement MinecraftAdapter**

Create `internal/adapter/minecraft.go` with `init()` self-registration. Maps profile to TYPE env var, sets EULA=TRUE, VERSION, and optional server settings (max players, MOTD, difficulty, mode, PVP, online mode). Returns TCP probes on port 25565. DataPath returns `/data`.

- [ ] **Step 4: Write adapter tests**

Test: adapter registry lookup, unknown game type error, Minecraft env generation (profile mapping, EULA, server config), probes, data path.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/adapter/... -v
```

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "feat: implement game adapter interface, registry, and Minecraft adapter"
```

---

### Task 5: Implement Resource Builders (PVC, StatefulSet, Service)

- [ ] **Step 1: Create PVC builder**

Create `internal/reconciler/pvc.go`:

`BuildPVC(gs)` — builds a PVC named `<gs.Name>-data` with labels, storage size, access modes, optional storage class.

- [ ] **Step 2: Create StatefulSet builder**

Create `internal/reconciler/statefulset.go`:

`BuildStatefulSet(gs)` — uses the game adapter to get env, probes, data path. Builds a StatefulSet with:
- Single replica
- Owner reference set by caller
- Volume claim template for data
- Container with image, env, ports, volume mounts, probes, security context, resources
- Headless service name `<gs.Name>-headless`
- PodSpec with scheduling (node selector, affinity, tolerations)
- TerminationGracePeriodSeconds = 120
- `mustConvertIntStr` helper

Also `buildSecurityContext(gs)` — constructs SecurityContext from SecuritySpec.

- [ ] **Step 3: Create Service builder**

Create `internal/reconciler/service.go`:

`BuildService(gs)` — builds the external Service (LoadBalancer/NodePort/ClusterIP).
`BuildHeadlessService(gs)` — builds the headless Service for StatefulSet DNS.
`GameServerLabels(gs)` — returns standard Kubernetes labels.

- [ ] **Step 4: Create labels helper**

Add `GameServerLabels()` in `service.go` (already included above).

- [ ] **Step 5: Write builder tests**

Test PVC size/name/namespace, Service ports/type, headless service ClusterIP=None, StatefulSet replica count/image/env/container name/service name.

- [ ] **Step 6: Run tests**

```bash
go test ./internal/reconciler/... -v
```

- [ ] **Step 7: Commit**

```bash
git add -A && git commit -m "feat: implement PVC, StatefulSet, and Service resource builders"
```

---

### Task 6: Implement Status and Finalizer Helpers

- [ ] **Step 1: Create status helper**

Create `internal/reconciler/status.go`:

```go
func SetCondition(gs *gamesv1alpha1.GameServer, condType string, status metav1.ConditionStatus, reason, message string)
func SetReady(gs *gamesv1alpha1.GameServer, ready bool, reason, message string)
func SetPhase(gs *gamesv1alpha1.GameServer, phase gamesv1alpha1.GameServerPhase)
func UpdateAddress(gs *gamesv1alpha1.GameServer, svc *corev1.Service)
```

These helpers manage `gs.Status.Conditions` using `metav1.SetCondition`, update Phase, Ready, Endpoint, Address, ObservedGeneration.

- [ ] **Step 2: Create finalizer helper**

Create `internal/reconciler/finalizer.go`:

```go
func AddFinalizer(ctx context.Context, c client.Client, gs *gamesv1alpha1.GameServer) (bool, error)
func RemoveFinalizer(ctx context.Context, c client.Client, gs *gamesv1alpha1.GameServer) error
func HasFinalizer(gs *gamesv1alpha1.GameServer) bool
func HandleFinalization(ctx context.Context, c client.Client, gs *gamesv1alpha1.GameServer, gameAdapter adapter.GameAdapter) error
```

Finalization logic: mark phase Stopping, trigger graceful shutdown (send stop command via exec or signal), wait for clean exit.

- [ ] **Step 3: Write helper tests**

Test: AddFinalizer adds finalizer string, RemoveFinalizer removes it, HasFinalizer checks correctly, SetCondition updates conditions correctly, SetPhase updates phase, UpdateAddress extracts ExternalIP.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/reconciler/... -v
```

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: implement status condition and finalizer helpers"
```

---

### Task 7: Implement the GameServer Controller Reconcile Loop

- [ ] **Step 1: Implement the full Reconcile loop**

Replace the scaffolded `internal/controller/gameserver_controller.go` with the full reconciliation:

```
Reconcile(ctx, req):
  1. Fetch GameServer
  2. If not found, return OK (deleted)
  3. If deletion timestamp set, run finalization
  4. If no finalizer, add it
  5. Get game adapter
  6. Reconcile PVC (if not exists, create)
  7. Reconcile Headless Service (if not exists, create)
  8. Reconcile StatefulSet (if not exists, create; if exists, update)
  9. Reconcile external Service (if not exists, create)
  10. Update status conditions (Ready, Progressing)
  11. Update address from Service
  12. Return requeue after 30s for status polling
```

Use `controllerutil.CreateOrUpdate` for each resource. Set owner references. Use `controllerutil.AddFinalizer` pattern.

- [ ] **Step 2: Implement finalization path**

When GameServer is being deleted:
1. Set phase to Stopping
2. Scale StatefulSet replicas to 0 (graceful shutdown)
3. Wait until pods terminated (requeue)
4. Remove finalizer

- [ ] **Step 3: Add RBAC markers**

Add `+kubebuilder:rbac` markers for StatefulSets, Services, PVCs, GameServers, and their status subresources.

- [ ] **Step 4: Setup controller in main.go**

Ensure `cmd/main.go` registers the GameServer controller and webhook with the manager.

- [ ] **Step 5: Generate manifests**

```bash
make manifests
```

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "feat: implement GameServer controller reconcile loop with finalizers"
```

---

### Task 8: Write Controller Integration Tests with envtest

- [ ] **Step 1: Create envtest-based integration test suite**

Create `internal/controller/gameserver_controller_test.go` (or `controllers/suite_test.go` if kubebuilder uses that path).

Setup envtest with `sigs.k8s.io/controller-runtime/pkg/envtest`.

- [ ] **Step 2: Write reconciliation test: GameServer creates StatefulSet**

Test that creating a GameServer results in a StatefulSet being created with correct image, env vars, and labels.

- [ ] **Step 3: Write reconciliation test: GameServer creates Service**

Test that creating a GameServer creates both headless and external services with correct ports.

- [ ] **Step 4: Write reconciliation test: GameServer creates PVC**

Test that PVC is created with correct size and storage class.

- [ ] **Step 5: Write reconciliation test: Status updates**

Test that after StableSet is ready, GameServer status is updated with Phase=Running, Ready=true, and Address is populated.

- [ ] **Step 6: Write reconciliation test: Finalizer and deletion**

Test that creating a GameServer adds a finalizer, and deleting it triggers the finalization path before removing the finalizer.

- [ ] **Step 7: Run integration tests**

```bash
make test
```

- [ ] **Step 8: Commit**

```bash
git add -A && git commit -m "test: add envtest integration tests for GameServer controller"
```

---

### Task 9: Create Sample CRD and Install/Run Locally

- [ ] **Step 1: Create a sample GameServer manifest**

Create `config/samples/games_v1alpha1_gameserver.yaml`:

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServer
metadata:
  name: minecraft-survival
  namespace: default
spec:
  game:
    type: minecraft
    version: "1.21"
    profile: paper
  runtime:
    image: itzg/minecraft-server:latest
  resources:
    requests:
      cpu: 500m
      memory: 2Gi
    limits:
      cpu: "2"
      memory: 4Gi
  storage:
    size: 20Gi
    storageClass: longhorn
  network:
    ports:
      - name: minecraft
        port: 25565
        protocol: TCP
    serviceType: LoadBalancer
  server:
    maxPlayers: 20
    motd: "GobeHost Minecraft Server"
    difficulty: normal
    gameMode: survival
    pvp: false
    onlineMode: true
  security:
    runAsNonRoot: true
    dropAllCapabilities: true
    seccompProfile: RuntimeDefault
```

- [ ] **Step 2: Install CRDs into a KinD/test cluster**

```bash
make install
```

- [ ] **Step 3: Run the controller locally**

```bash
make run
```

- [ ] **Step 4: Apply the sample and verify**

```bash
kubectl apply -f config/samples/games_v1alpha1_gameserver.yaml
kubectl get gameserver
kubectl describe gameserver minecraft-survival
```

- [ ] **Step 5: Verify resources created**

```bash
kubectl get statefulset,svc,pvc -l games.gobehost.com/game-id=minecraft-survival
```

- [ ] **Step 6: Clean up**

```bash
kubectl delete gameserver minecraft-survival
```

Verify finalizer runs and resources are cleaned up.

- [ ] **Step 7: Commit sample**

```bash
git add -A && git commit -m "feat: add sample GameServer manifest and verify local install"
```

---

### Task 10: Final Verification and Build

- [ ] **Step 1: Run all tests**

```bash
make test
```

- [ ] **Step 2: Run linter**

```bash
make lint
```

If lint target doesn't exist yet, add golangci-lint:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
make lint
```

- [ ] **Step 3: Build the manager binary**

```bash
make build
```

- [ ] **Step 4: Build the Docker image**

```bash
make docker-build IMG=gobehost/operator:latest
```

- [ ] **Step 5: Verify CRD manifests are complete**

```bash
make manifests
cat config/crd/bases/games.gobehost.com_gameservers.yaml | head -50
```

Confirm all fields, validations, and defaultings are in the CRD schema.

- [ ] **Step 6: Final commit**

```bash
git add -A && git commit -m "chore: verify all tests pass and build succeeds"
```

---

## Self-Review Checklist

- **Spec coverage:** Every CRD field from the plan (Game, Runtime, Resources, Storage, Network, Server, Backup, Scheduling, Security) has types, defaults, and validation. ✓
- **Placeholder scan:** All steps have concrete code or commands. No TBD/TODO placeholders. ✓
- **Type consistency:** GameServerSpec, GameServerStatus, adapter interface, builder functions all use consistent types throughout. ✓
- **Gap check:** The plan covers CRD types, webhooks, adapters, builders, controller, finalization, integration tests, and local verification. Missing: Backup/TrafficPolicy CRDs (Phase 2), metrics (Phase 2), fleet (Phase 2). These are explicitly out of scope for Phase 1 per plan.md.