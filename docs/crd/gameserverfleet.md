# GameServerFleet

## Overview

`GameServerFleet` manages the lifecycle of a single `GameServer` instance using a SaaS provisioning model where **exactly one replica is allocated per customer tenant**. Unlike traditional fleet controllers that scale horizontal replica counts, `GameServerFleet` always maintains one GameServer and provides controlled rollout strategies to update it without downtime.

Each `GameServerFleet`:

- Owns exactly one `GameServer` resource
- Manages a stable `Service` that routes traffic to the active GameServer
- Tracks the GameServer's readiness and reflects it in fleet-level status
- Supports rolling updates (zero-downtime) or recreate updates (stop-then-start)
- Records rollout history for observability

```txt
GameServerFleet
  ├── GameServer (owned, 1:1)
  └── Service     (owned, stable endpoint)
```

**Short name:** `gsf`  
**API Group:** `games.gobehost.com`  
**API Version:** `v1alpha1`  
**Scope:** Namespaced

---

## Spec Reference

```yaml
spec:
  replicas: 1
  strategy:
    type: RollingUpdate          # RollingUpdate | Recreate
  template:
    metadata: {}
    spec: {}                      # Full GameServerSpec
```

### replicas

| Field | Value |
|-------|-------|
| Type | `integer` |
| Default | `1` |
| Minimum | `1` |
| Maximum | `1` |

Hardcoded to `1`. This enforces the one-GameServer-per-fleet SaaS model. Setting any other value is rejected by the webhook. Once set, the field is immutable.

### strategy

| Field | Value |
|-------|-------|
| Type | `object` |
| Default | `{type: RollingUpdate}` |

Controls how template changes are applied to the owned GameServer.

| `strategy.type` | Behavior |
|---|---|
| `RollingUpdate` | Creates a new GameServer, waits for it to become ready, then drains and removes the old one. Zero-downtime. |
| `Recreate` | Deletes the current GameServer before creating a new one. Incurs downtime. |

### template

| Field | Value |
|-------|-------|
| Type | `object` |
| Required | Yes |

The `GameServerTemplate` to stamp out. Contains two sub-fields:

#### template.metadata

Standard Kubernetes object metadata (labels, annotations). Applied to the owned GameServer. The `name` field is ignored — the controller derives the GameServer name from the fleet name plus a suffix.

#### template.spec

A full `GameServerSpec` embedded verbatim. See the [GameServer CRD reference](gameserver.md) for all sub-fields (`game`, `runtime`, `resources`, `storage`, `network`, `security`).

---

## Status Reference

```yaml
status:
  phase: Available
  observedGeneration: 1
  readyReplicas: 1
  currentGameServer: minecraft-survival-gs
  updatedGameServer: ""
  conditions: []
  history: []
```

### phase

| Phase | Meaning |
|-------|--------|
| `Progressing` | The fleet is creating a GameServer or performing a rollout. |
| `Available` | The owned GameServer is Ready and serving traffic. |
| `Degraded` | The owned GameServer exists but is not Ready (e.g. starting up). |
| `Failed` | The owned GameServer has entered the `Failed` phase. |

### observedGeneration

The last `metadata.generation` the controller has reconciled. Used to determine whether the spec has changed since the last observation.

### readyReplicas

Count of GameServers in `Ready=true` state. Always `0` or `1`.

### currentGameServer

Name of the active (current) GameServer owned by this fleet.

### updatedGameServer

Name of the incoming GameServer during a rolling update. Empty when no update is in progress.

### conditions

A list of `metav1.Condition` entries using `type` as the map key.

| Type | Description |
|------|-------------|
| `Available` | `True` when the GameServer is Ready; `False` otherwise. |
| `Progressing` | `True` during rollouts or initial creation; `False` when stable. |
| `Degraded` | `True` when the GameServer is not Ready or has Failed. |

### history

A list of `RolloutRecord` entries (capped at 10) tracking completed rollouts.

| Field | Description |
|-------|-------------|
| `startedAt` | Timestamp when the rollout began. |
| `completedAt` | Timestamp when the rollout finished (absent while in progress). |
| `fromVersion` | The previous game version. |
| `toVersion` | The target game version. |
| `result` | `Success` or `Failed`. |
| `message` | Human-readable summary. |

---

## Phases

The fleet transitions through four phases:

```txt
                    ┌──────────────┐
     Create/Update  │ Progressing   │
 ─────────────────►│               │
                    └───────┬───────┘
                            │
                   GameServer Ready=true
                            │
                    ┌───────▼───────┐
                    │   Available    │◄──── Reconcile loop (steady)
                    └───────┬───────┘
                            │
                GameServer not Ready
                            │
                    ┌───────▼───────┐
                    │   Degraded    │
                    └───────┬───────┘
                            │
                GameServer Phase=Failed
                            │
                    ┌───────▼───────┐
                    │    Failed     │
                    └───────────────┘
```

| Phase | Entry Condition | Exit Condition |
|-------|----------------|----------------|
| **Progressing** | Fleet created, GameServer not yet Ready, or rollout starting | GameServer transitions to Ready |
| **Available** | GameServer is Ready and serving traffic | Template change detected or GameServer becomes unready |
| **Degraded** | GameServer exists but is not Ready (and not Failed) | GameServer becomes Ready or transitions to Failed |
| **Failed** | GameServer has `phase: Failed` | Controller deletes and recreates the GameServer, returning to Progressing |

When a GameServer enters `Failed`, the controller automatically deletes it and creates a replacement, transitioning the fleet back to `Progressing`.

---

## Rolling Update Lifecycle

The `RollingUpdate` strategy provides zero-downtime rollouts by running two GameServers simultaneously during the transition. The process is driven by an annotation-based state machine on the fleet object.

### State Machine

```txt
                          Template change detected
                                   │
                          ┌────────▼────────┐
                          │  Create new GS   │
                          │  Set phase        │
                          │  annotation =     │
                          │  waiting-for-ready│
                          └────────┬─────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │     waiting-for-ready          │
                    │  Poll new GS until Ready=true  │
                    └──────────────┬─────────────────┘
                                   │
                          New GS is Ready
                                   │
                    ┌──────────────▼──────────────┐
                    │     Annotate old GS with      │
                    │     games.gobehost.com/drain  │
                    │     Set phase annotation =     │
                    │     draining-old               │
                    └──────────────┬─────────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │       draining-old             │
                    │  Wait for old GS to stop       │
                    │  (Phase=Stopped or deleted)    │
                    └──────────────┬─────────────────┘
                                   │
                          Old GS stopped
                                   │
                    ┌──────────────▼──────────────┐
                    │   Cutover:                     │
                    │   • Delete old GS              │
                    │   • Update Service selector    │
                    │   • Set status.currentGameServer│
                    │   • Clear phase annotation      │
                    │   • Record rollout in history   │
                    │   • Phase → Available           │
                    └─────────────────────────────────┘
```

### Annotation: `games.gobehost.com/update-phase`

| Value | Meaning |
|-------|---------|
| `waiting-for-ready` | A new GameServer has been created. The controller polls until it reports `Ready=true`. |
| `draining-old` | The new GameServer is ready. The old GameServer has been annotated with `games.gobehost.com/drain=true`. The controller waits for it to reach `Phase=Stopped` or be deleted. |

Absence of this annotation means the fleet is in steady state.

### Annotation: `games.gobehost.com/template-hash`

An FNV-1a hash of the `GameServerTemplate.Spec`, stored on each GameServer. The controller compares this hash against the fleet's current template to detect drift and trigger rollouts.

### Traffic Cutover

During a rolling update, the fleet's `Service` selector is only updated to point at the new GameServer once the cutover is complete (after the old GameServer has stopped). This ensures clients connected to the stable Service endpoint are only directed to the new GameServer after it is confirmed ready.

---

## Recreate Strategy

The `Recreate` strategy tears down the current GameServer before creating a replacement:

1. Template change is detected (hash mismatch).
2. The current GameServer is deleted immediately.
3. `status.currentGameServer` is cleared, `status.phase` is set to `Progressing`.
4. A new GameServer is created from the updated template on the next reconcile.
5. The fleet transitions through `Progressing` → `Available` as the new GameServer becomes ready.

This strategy incurs downtime (the period between the old GameServer being deleted and the new one becoming Ready). Use it when the game server cannot run two instances concurrently (e.g. shared storage with exclusive-write semantics).

---

## Stable Service Pattern

Each `GameServerFleet` manages a `Service` named after the fleet (e.g. `minecraft-survival`). This Service:

- Uses the `games.gobehost.com/fleet-name` label as a selector to route traffic to the active GameServer
- Is created when the first GameServer is provisioned
- Has its selector updated during rolling update cutover to point at the new GameServer
- Inherits the `ServiceType` and port definitions from `template.spec.network`
- Is deleted when the fleet is deleted (via owner references)

Clients should connect to this stable Service endpoint rather than directly to the GameServer pod.

---

## Webhook Defaults and Validations

### Defaulting (Mutating Webhook)

Applied on `CREATE` and `UPDATE`:

| Field | Default |
|-------|---------|
| `spec.replicas` | `1` (if set to `0` or omitted) |
| `spec.strategy.type` | `RollingUpdate` (if empty) |
| `spec.template.spec.runtime.imagePullPolicy` | `IfNotPresent` (if empty) |
| `spec.template.spec.storage.accessModes` | `[ReadWriteOnce]` (if empty) |
| `spec.template.spec.network.serviceType` | `LoadBalancer` (if empty) |

### Validation (Validating Webhook)

Applied on `CREATE`:

| Rule | Detail |
|------|--------|
| `spec.replicas` must equal `1` | Rejects any other value. |
| `spec.template.spec.game.type` is required | Must not be empty. |
| `spec.template.spec.game.version` is required | Must not be empty. |
| `spec.template.spec.runtime.image` is required | Must not be empty. |
| `spec.template.spec.network.ports` is required | At least one port must be defined. |
| `spec.template.spec.storage.size` is required | Must not be empty (zero value). |

Applied on `UPDATE` (in addition to all create rules):

| Rule | Detail |
|------|--------|
| `spec.replicas` is immutable | Changing the replicas field is forbidden. |

---

## Example YAML

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServerFleet
metadata:
  name: minecraft-survival
  namespace: default
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
        env:
          - name: PAPER_BUILD
            value: "66"
      resources:
        requests:
          cpu: "1"
          memory: 2Gi
        limits:
          cpu: "2"
          memory: 4Gi
      storage:
        size: 10Gi
        storageClass: longhorn
      network:
        ports:
          - name: minecraft
            port: 25565
            protocol: TCP
        serviceType: LoadBalancer
      security:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 1000
        fsGroup: 1000
```

This creates a fleet named `minecraft-survival` that manages a single Minecraft Paper server. When the template changes (e.g. a new `version`), the controller performs a rolling update: it creates a new GameServer, waits for it to become Ready, drains the old one, then cuts over traffic via the stable Service.