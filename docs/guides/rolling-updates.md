# Rolling Updates

GameServerFleet supports two update strategies: **RollingUpdate** (default) and **Recreate**.

## RollingUpdate Strategy

Rolling updates provide zero-downtime upgrades by creating a new GameServer alongside the existing one, then switching traffic once the new server is healthy.

### How It Works

1. **Detect change**: The controller computes a hash of `spec.template.spec`. When the hash differs from the current GameServer's annotation, an update begins.

2. **Phase A — Spawn**: A new GameServer is created with a hash-suffixed name (e.g., `minecraft-prod-f0fb6531`). The fleet's `status.updatedGameServer` is set, and the `games.gobehost.com/update-phase` annotation is set to `waiting-for-ready`.

3. **Phase B — Cutover**: Once the new GameServer reaches `Running` + `Ready`, the old GameServer receives a `games.gobehost.com/drain=true` annotation. The fleet transitions to `draining-old` phase.

4. **Completion**: When the old GameServer reaches `Stopped` phase, it is deleted. The stable Service selector is updated to point at the new GameServer. The fleet's `status.currentGameServer` is updated, and the rollout is recorded in `status.history`.

### Stable Service

The fleet manages a Service named after the fleet (e.g., `minecraft-prod`). This Service's selector is updated at cutover time, so external consumers (Traefik IngressRouteTCP, clients) should target the fleet Service, not the individual GameServer Service.

### Rollout History

Each completed rollout is recorded in `status.history` (up to 10 entries):

```yaml
status:
  history:
    - startedAt: "2026-06-01T10:00:00Z"
      completedAt: "2026-06-01T10:05:00Z"
      fromVersion: "1.21"
      toVersion: "1.22"
      result: Success
      message: "Rolling update to version 1.22 completed"
```

### Example

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServerFleet
metadata:
  name: minecraft-prod
spec:
  replicas: 1
  strategy:
    type: RollingUpdate  # default
  template:
    spec:
      game:
        type: minecraft
        version: "26.1.2"  # Change this to trigger a rolling update
      # ... rest of spec
```

Trigger an update by changing `spec.template.spec.game.version` (or any other template field):

```bash
kubectl patch gameserverfleet minecraft-prod --type merge -p \
  '{"spec":{"template":{"spec":{"game":{"version":"1.22"}}}}}'
```

## Recreate Strategy

The Recreate strategy deletes the current GameServer immediately, then creates a new one from the updated template. This results in downtime but is simpler and appropriate for dev/staging environments.

```yaml
spec:
  strategy:
    type: Recreate
```

## Monitoring Updates

```bash
# Watch fleet status
kubectl get gameserverfleet -w

# Check rollout history
kubectl get gameserverfleet minecraft-prod -o jsonpath='{.status.history}'

# Check update phase
kubectl get gameserverfleet minecraft-prod -o jsonpath='{.metadata.annotations.games\.gobehost\.com/update-phase}'
```

## PVC Behavior

PersistentVolumeClaims are **never** deleted during a rolling update. The new GameServer reuses the same PVC name (derived from the fleet name), ensuring data continuity across updates.