# Adapter Pattern

The GobeHost operator uses the **adapter pattern** to separate game-specific configuration from the generic reconciliation logic.

## Interface

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

## How It Works

1. The `GameServerReconciler` calls `adapter.Get(gs)` with the GameServer resource
2. The registry looks up the adapter by `spec.game.type`
3. If found, the reconciler uses the adapter to build the container spec (env vars, probes, security context)
4. If not found, the GameServer is set to `Failed` phase with an `AdapterNotFound` condition

## Self-Registration

Adapters register themselves via `init()`:

```go
func init() {
    Register(&MinecraftAdapter{})
}
```

This is imported via a blank import in `cmd/main.go`:
```go
_ "github.com/gobehost/operator/internal/adapter"
```

## Adding a New Adapter

See the [Custom Game Guide](../guides/custom-game.md) for step-by-step instructions.

## Existing Adapters

### MinecraftAdapter

- **Type**: `minecraft`
- **Image**: `itzg/minecraft-server`
- **Profile mapping**: `paper` → TYPE=PAPER, `forge` → TYPE=FORGE, etc.
- **Probes**: TCP socket on port 25565 (readiness: 30s/10s, liveness: 120s/30s)
- **Data path**: `/data`
- **Default security**: UID/GID/FSGroup 1000

## Registry

The `registry.go` provides:

| Function | Description |
|----------|-------------|
| `Register(a GameAdapter)` | Register an adapter |
| `Get(gs *GameServer)` | Look up adapter by game type, returns error if not found |
| `KnownGames()` | List registered game type names |