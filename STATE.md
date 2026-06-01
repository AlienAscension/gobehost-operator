# GobeHost Operator - Project State

_Last updated: 2026-06-01_

## Project Overview

Kubernetes operator for managing game servers. Built with kubebuilder 4.13.1, Go 1.26.3, controller-runtime, Ginkgo/Gomega tests.

- **Domain**: `gobehost.com`
- **Repo**: `github.com/gobehost/operator`
- **Git remote**: `git@github.com:AlienAscension/gobehost-operator.git`
- **Container image**: `linusdb/gobehost:v0.1.0` (also `latest`)
- **Container tool**: podman (not docker)
- **Cluster**: Talos Linux, Cilium CNI, Traefik ingress, Longhorn storage, FluxCD

## Live Deployment

A working Minecraft server is deployed and accessible at `192.168.0.200:25565`.

- **GameServer**: `minecraft-survival` in `default` namespace
- **MC version**: Paper 26.1.2 build 66 (Java 25, protocol 775)
- **Container image**: `itzg/minecraft-server:java25`
- **Status**: `phase: Running`, `ready: true`
- **World data**: persisted on Longhorn PVC `minecraft-survival-data`

## Key Architecture Decisions

| Decision | Rationale |
|---|---|
| Manual Helm chart (not kubebuilder plugin) | User preference |
| Podman for builds | User preference |
| `runAsNonRoot: true` with UID/GID/FSGroup 1000 | Matches itzg container UID |
| No `dropAllCapabilities: true` in sample | Caused spark profiler JVM SIGSEGV |
| `itzg/minecraft-server:java25` | MC 26.1.2 requires Java 25 |
| Traefik IngressRouteTCP (not NodePort) | Clean TCP routing through existing LB |
| `VERSION` env = MC game version (e.g. "26.1.2") | Paper build selected automatically or via `PAPER_BUILD` env |
| `docs/superpowers/` gitignored | Internal planning, not for distribution |
| GameServerFleet stable Service | Fleet owns a Service named after the fleet, updated at cutover time; decouples routing from GameServer lifecycle |

## Project Structure

```
cmd/main.go                         Manager entry point
api/v1alpha1/
  gameserver_types.go               GameServer CRD schema
  gameserverfleet_types.go          GameServerFleet CRD schema (replicas:1, rolling update, status)
  zz_generated.deepcopy.go          Auto-generated (DO NOT EDIT)
internal/adapter/
  adapter.go                        GameAdapter interface
  minecraft.go                      MinecraftAdapter (env mapping, probes, security context)
  registry.go                       Adapter registry
internal/controller/
  gameserver_controller.go          GameServer reconciliation
  gameserverfleet_controller.go     GameServerFleet reconciliation (rolling update, steady state, finalization)
internal/reconciler/
  statefulset.go                    BuildStatefulSet (PodSecurityContext + ContainerSecurityContext)
  service.go                        BuildService + BuildHeadlessService
  pvc.go                            BuildPVC
  fleetservice.go                   BuildFleetService (stable Service for fleet)
  status.go                         Status condition helpers
  finalizer.go                      Finalizer logic
internal/webhook/v1alpha1/
  gameserver_webhook.go              Defaulting + validation (26 tests, 100% coverage)
  gameserverfleet_webhook.go         Defaulting + validation for GameServerFleet
charts/gobehost-operator/            Helm chart (manual, CRDs must sync from config/crd/bases/)
config/
  crd/bases/                        Generated CRDs (DO NOT EDIT)
  samples/
    games_v1alpha1_gameserver.yaml   Current sample: MC 26.1.2 paper, java25, PAPER_BUILD=66
    games_v1alpha1_gameserverfleet.yaml  Fleet sample
    minecraft-ingressroute.yaml      Traefik IngressRouteTCP for minecraft
  rbac/role.yaml                     Generated RBAC (DO NOT EDIT)
dist/install.yaml                    Non-Helm installation bundle
```

## Auto-Generated Files (NEVER EDIT)

- `config/crd/bases/*.yaml` — run `make manifests`
- `config/rbac/role.yaml` — run `make manifests`
- `**/zz_generated.*.go` — run `make generate`
- `PROJECT` — managed by kubebuilder CLI

## GameServerFleet CRD

Manages lifecycle of a single GameServer (replicas: 1) per customer (SaaS provisioning model).

```yaml
spec:
  replicas: 1                    # hardcoded for SaaS model
  strategy:
    type: RollingUpdate|Recreate
  template:
    metadata:
      labels: {}
      annotations: {}
    spec:
      <full GameServerSpec embedded verbatim>
status:
  phase: Progressing|Available|Degraded|Failed
  observedGeneration: int64
  readyReplicas: 0|1
  conditions: []metav1.Condition
  currentGameServer: string       # name of active GS
  updatedGameServer: string       # name of incoming GS during rollout
  history: []RolloutRecord        # last 10 completed rollouts
```

**Rolling update state machine**:
1. Template hash changes → create new GS with hash-suffixed name
2. Annotate fleet with `update-phase: waiting-for-ready`
3. New GS ready → annotate old GS with `drain: true`, set `update-phase: draining-old`
4. Old GS stopped/deleted → complete cutover, update stable Service selector, record history

**Stable Service**: Fleet owns a Service named after the fleet. Selector points to the active GameServer. Updated at cutover time to point to the new GS.

## Test Coverage

| Package | Coverage |
|---|---|
| internal/adapter | 97.6% |
| internal/controller | 47.4% |
| internal/reconciler | 82.4% |
| internal/webhook/v1alpha1 | 99.0% |

Note: `make test` exits with error due to `covdata` tool not found (Go toolchain issue), but all 4 test suites pass.

## Common Commands

```bash
# Regenerate after editing types/markers
make manifests generate

# Run tests
make test

# Build & push container
make docker-build docker-push IMG=linusdb/gobehost:v0.1.0

# Deploy to cluster
make deploy IMG=linusdb/gobehost:v0.1.0

# Apply CRDs (after schema changes)
kubectl apply --server-side --force-conflicts -k config/crd

# Apply sample
kubectl apply -f config/samples/games_v1alpha1_gameserver.yaml -f config/samples/minecraft-ingressroute.yaml

# Helm
make helm-lint
make helm-package

# Check GameServer status
kubectl get gameserver minecraft-survival -o wide

# View server logs
kubectl logs minecraft-survival-0 -f
```

## RBAC

The controller has RBAC for:
- `gameservers` (CRUD + status + finalizers) under `games.gobehost.com`
- `gameserverfleets` (CRUD + status + finalizers) under `games.gobehost.com`
- `services` (CRUD) under `games.gobehost.com` fleet controller
- `statefulsets`, `services`, `persistentvolumeclaims` under `apps`/`core`
- `leases` under `coordination.k8s.io`
- `events` under core API group `""` (not `events.k8s.io`)

## Known Issues & History

| Issue | Resolution |
|---|---|
| Java 25 crash with spark profiler | Switched to `itzg/minecraft-server:java25`, removed `dropAllCapabilities: true` |
| RBAC `events.k8s.io` group wrong | Fixed to core `""` group |
| RBAC missing leases | Added `coordination.k8s.io/leases` |
| Dockerfile `./cmd` build path | Fixed in Dockerfile |
| .dockerignore excluding needed files | Fixed |
| MC 1.21.5 "outdated server" error | Updated to MC 26.1.2 + Java 25 image |
| World format migration MC 1.21.5→26.1.2 | Automatic on first boot (WorldFolderMigration) |
| `make test` exit code 1 | `covdata` tool not found; tests themselves all pass |

## Next Steps (Ideas)

- Add more game adapters (Valheim, CS2, Terraria, etc.)
- Add Backup CRD / backup logic
- Implement scheduling policies (restart, update windows)
- Sync CRDs to Helm chart when types changes
- Fix `make test` covdata issue
- Improve controller test coverage (currently 47.4%)
- Add GameServerFleet e2e tests