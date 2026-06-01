# Architecture Overview

The GobeHost operator is a Kubernetes controller built with [kubebuilder](https://book.kubebuilder.io/) that manages game server lifecycle using Kubernetes-native declarative APIs.

## High-Level Architecture

```
                    ┌─────────────────────┐
                    │   GameServerFleet    │
                    │   (CRD, replicas:1)  │
                    └────────┬────────────┘
                             │ owns
                    ┌────────▼────────────┐
                    │    GameServer        │
                    │   (CRD, lifecycle)  │
                    └────────┬────────────┘
                             │ reconciles
              ┌──────────────┼──────────────┐
              │              │              │
     ┌────────▼───┐  ┌──────▼──────┐  ┌───▼────────┐
     │ StatefulSet│  │  Services   │  │    PVC     │
     │ (game pod) │  │(headless +  │  │ (data vol) │
     │            │  │ external)   │  │            │
     └────────────┘  └─────────────┘  └────────────┘
```

## Core Components

### Controllers

| Controller | Manages | Purpose |
|-----------|---------|---------|
| GameServerReconciler | GameServer → STS + Services + PVC | Provisions and manages a single game server |
| GameServerFleetReconciler | GameServerFleet → GameServer + Fleet Service | Lifecycle management with rolling updates |

### Adapters

The adapter pattern decouples game-specific logic from the controller:

```
GameServerReconciler
    │
    ├── adapter.Get(gs) ──→ MinecraftAdapter
    │                        ├── Env() → container env vars
    │                        ├── Probes() → readiness/liveness probes
    │                        └── DataPath() → volume mount path
    │
    └── reconciler.BuildStatefulSet(gs) → uses adapter output
```

### Webhooks

Defaulting and validation webhooks enforce constraints:

- **GameServer**: defaults imagePullPolicy, serviceType, security context, port protocols
- **GameServerFleet**: defaults replicas to 1, strategy to RollingUpdate; validates replicas=1, required fields

## Lifecycle

### GameServer Phases

```
Pending → Provisioning → Running → Stopping → Stopped
                ↓                        ↑
              Failed ←──────────────────┘
```

- **Pending**: Initial state before reconciliation
- **Provisioning**: STS, Services, PVC being created
- **Running**: STS ready replicas ≥ 1
- **Failed**: Adapter not found or STS failed
- **Stopping/Stopped**: Deletion in progress, STS scaled to 0
- **BackupInProgress**: (Future) Backup in progress

### GameServerFleet Phases

```
Progressing → Available
    ↓              ↑
  Failed      Degraded
```

- **Progressing**: Initial provision or rolling update in progress
- **Available**: GameServer running and ready
- **Degraded**: GameServer exists but not ready
- **Failed**: GameServer in Failed phase, will be recreated

## Rolling Update Flow

```
Fleet spec change detected
        │
        ▼
Create new GS (<fleet>-<hash8>)
Set annotation: update-phase=waiting-for-ready
        │
        ▼ (poll every 10s)
New GS Ready?
        │
     No ──► requeue
     Yes
        │
        ▼
Annotate old GS: drain=true
Set annotation: update-phase=draining-old
        │
        ▼ (poll every 5s)
Old GS Stopped?
        │
     No ──► requeue
     Yes
        │
        ▼
Delete old GS
Update fleet Service selector → new GS
Record in history
Clear annotation
```

## Finalizers

Both CRDs use finalizers to ensure clean cleanup:

- **GameServer**: Scales STS to 0 replicas before removing finalizer, ensuring graceful shutdown
- **GameServerFleet**: Removes GameServer finalizers, deletes owned GameServers and Service before removing its own finalizer