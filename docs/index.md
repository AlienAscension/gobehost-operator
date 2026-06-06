# GobeHost Operator

![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![Kubernetes Version](https://img.shields.io/badge/Kubernetes-1.35-326CE5?logo=kubernetes&logoColor=white)
![License](https://img.shields.io/badge/License-Apache%202.0-green)

A Kubernetes operator for declaratively managing game servers. Define your game server infrastructure in code, let the operator handle the rest.

## What it does

GobeHost Operator provides three custom resources for managing game server lifecycles on Kubernetes:

| CRD | Purpose |
|---|---|
| **GameServer** | Manages an individual game server instance (Minecraft, Valheim, Terraria, etc.) |
| **GameServerFleet** | Manages the lifecycle of GameServers with rolling updates, 1 replica per customer (SaaS model) |
| **GameServerBackup** | Schedules and manages backups of game data to S3-compatible storage |

## Key features

- **Declarative management** -- Define game servers as Kubernetes resources; the controller handles StatefulSets, Services, PVCs, and IngressRoutes
- **StatefulSet-backed persistence** -- World data survives pod restarts and node failures
- **Scheduled backups** -- `GameServerBackup` backs up PVC data to S3-compatible storage with CronJob-based scheduling, retention, and backup-on-delete
- **Adapter pattern** -- Pluggable game adapters translate a unified `GameServerSpec` into game-specific container configuration
- **Rolling updates with zero-downtime cutover** -- `GameServerFleet` spins up a new server, waits for readiness, then cuts traffic over before tearing down the old one
- **Traefik IngressRouteTCP integration** -- Native TCP routing for game traffic through Traefik

## Get started in 5 minutes

### Prerequisites

- A Kubernetes cluster (v1.35+)
- `kubectl` configured to talk to your cluster

### Install

=== "Kustomize"

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/AlienAscension/gobehost-operator/main/dist/install.yaml
    ```

=== "Helm"

    ```bash
    helm install gobehost-operator ./charts/gobehost-operator \
      --namespace gobehost-operator-system \
      --create-namespace
    ```

### Deploy a Minecraft server

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
    image: itzg/minecraft-server:java25
  resources:
    requests:
      cpu: 500m
      memory: 2Gi
    limits:
      cpu: "2"
      memory: 4Gi
  storage:
    size: 20Gi
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
```

```bash
kubectl apply -f minecraft.yaml
kubectl get gameserver minecraft-survival
```

### Deploy a fleet with rolling updates

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServerFleet
metadata:
  name: minecraft-survival
spec:
  replicas: 1
  strategy:
    type: RollingUpdate
  template:
    spec:
      game:
        type: minecraft
        version: "26.1.2"
        profile: paper
      runtime:
        image: itzg/minecraft-server:java25
      storage:
        size: 10Gi
      network:
        ports:
          - name: minecraft
            port: 25565
            protocol: TCP
```

Change `spec.template.spec.game.version` and the fleet handles the rolling update: starts a new GameServer, waits for readiness, cuts traffic over, then removes the old one.

```bash
kubectl apply -f fleet.yaml
kubectl get gameserverfleet minecraft-survival
```

## Next steps

- [:material-rocket-launch: Installation guide](getting-started/installation.md) -- detailed install options
- [:material-game-pad: Deploying Minecraft](guides/minecraft.md) -- full Minecraft server walkthrough
- [:material-swap-vertical: Rolling updates](guides/rolling-updates.md) -- zero-downtime update strategy
- [:material-backup: Backups](guides/backups.md) -- scheduled backups to S3-compatible storage
- [:material-puzzle: CRD reference](crd/gameserver.md) -- full API documentation
- [:material-architecture: Architecture](architecture/overview.md) -- how the controller works