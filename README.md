# GobeHost Operator

A Kubernetes operator for managing game server lifecycles. Defines a `GameServer` CRD that declaratively provisions StatefulSets, persistent storage, and networking for any supported game — with graceful shutdown, status conditions, and an extensible game adapter pattern.

Currently supports **Minecraft** (vanilla, Paper, Forge, Fabric, Spigot, Bukkit). Designed to grow with adapters for Valheim, CS2, Rust, Terraria, Factorio, ARK, and custom containerized servers.

## Features

- **GameServer CRD** — declare a game server instance with game type, version, resources, storage, networking, and server config
- **Automatic provisioning** — StatefulSet, PVC, headless Service, and external Service created and managed by the controller
- **Game adapter pattern** — each game plugs in via a simple interface; add new games without touching the controller
- **Graceful shutdown** — finalizers ensure StatefulSets scale down cleanly before deletion
- **Status conditions** — `Ready`, `Provisioning`, `Phase` tracking via Kubernetes-standard conditions
- **Defaulting & validation webhooks** — sane defaults and immutability guarantees
- **Security by default** — runs as non-root (UID 1000), drops all capabilities, seccomp=RuntimeDefault
- **Helm chart** — install with a single `helm install`

## Quick Start

### Helm (recommended)

```bash
helm install gobehost-operator charts/gobehost-operator/ \
  --namespace gobehost-operator-system \
  --create-namespace
```

### Non-Helm (single YAML)

```bash
kubectl apply -f dist/install.yaml
```

### Create a Minecraft server

```bash
kubectl apply -f config/samples/games_v1alpha1_gameserver.yaml
```

Check status:

```bash
kubectl get gameserver
kubectl describe gameserver minecraft-survival
```

### Clean up

```bash
kubectl delete gameserver minecraft-survival
helm uninstall gobehost-operator -n gobehost-operator-system
```

## GameServer Example

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServer
metadata:
  name: minecraft-survival
spec:
  game:
    type: minecraft
    version: "26.1.2"
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
    gameMode: survival
    pvp: false
    onlineMode: true
  security:
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
    dropAllCapabilities: true
    seccompProfile: RuntimeDefault
```

## Prerequisites

- Kubernetes 1.31+
- [cert-manager](https://cert-manager.io/) (for webhook TLS)
- Go 1.26+ (for development)
- podman or docker (for building images)

## Development

```bash
# Run tests
make test

# Run locally (against current kubeconfig)
make run

# Build container image
make docker-build IMG=linusdb/gobehost:v0.1.0
make docker-push IMG=linusdb/gobehost:v0.1.0

# Regenerate CRDs after editing types
make manifests generate

# Lint
make lint-fix
```

## Adding a Game Adapter

1. Implement the `GameAdapter` interface in `internal/adapter/`:

```go
type GameAdapter interface {
    Name() string
    Env(gs *gamesv1alpha1.GameServer) []corev1.EnvVar
    Command(gs *gamesv1alpha1.GameServer) []string
    Args(gs *gamesv1alpha1.GameServer) []string
    Probes(gs *gamesv1alpha1.GameServer) (*corev1.Probe, *corev1.Probe)
    DataPath(gs *gamesv1alpha1.GameServer) string
    DefaultSecurityContext() *corev1.PodSecurityContext
}
```

2. Register it with `init()` — see `internal/adapter/minecraft.go`

3. Add the game type to the webhook validation allowlist (optional)

## Project Distribution

### Build the install bundle

```bash
make build-installer IMG=linusdb/gobehost:v0.1.0
```

Generates `dist/install.yaml` — a single YAML with CRDs, RBAC, Deployment, and Webhooks.

### Package the Helm chart

```bash
make helm-package
```

Outputs `dist/gobehost-operator-0.1.0.tgz`.

## Architecture

```
GameServer CR
      │
      ▼
GameServerReconciler
      │
      ├─► GameAdapter (minecraft, valheim, ...)
      │       └─► Env vars, probes, data path, security context
      │
      ├─► BuildPVC ──► PersistentVolumeClaim
      ├─► BuildService ──► Service (LoadBalancer/NodePort)
      ├─► BuildHeadlessService ──► Service (ClusterIP: None)
      └─► BuildStatefulSet ──► StatefulSet (1 replica)
```
