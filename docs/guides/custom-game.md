# Adding Game Adapters

The GobeHost operator uses an **adapter pattern** to support multiple game types. Each adapter maps GameServer CRD fields to the specific environment variables, commands, probes, and security contexts required by a game server image.

## Architecture

```
GameServer CRD → adapter.Get(gs) → MinecraftAdapter.Env(gs)
                                    MinecraftAdapter.Command(gs)
                                    MinecraftAdapter.Probes(gs)
                                    ...
```

Adapters self-register via `init()` in their file. The registry looks up an adapter by `spec.game.type`.

## Current Adapters

| Game | Type | Image | Status |
|------|------|-------|--------|
| Minecraft | `minecraft` | `itzg/minecraft-server` | Stable |

## Adding a New Adapter

### 1. Create the adapter file

Create `internal/adapter/<game>.go`:

```go
package adapter

import (
    corev1 "k8s.io/api/core/v1"
    gamesv1alpha1 "github.com/gobehost/operator/api/v1alpha1"
)

type MyGameAdapter struct{}

func (a *MyGameAdapter) Name() string {
    return "mygame"
}

func (a *MyGameAdapter) Env(gs *gamesv1alpha1.GameServer) []corev1.EnvVar {
    return []corev1.EnvVar{
        {Name: "MY_GAME_VERSION", Value: gs.Spec.Game.Version},
    }
}

func (a *MyGameAdapter) Command(gs *gamesv1alpha1.GameServer) []string {
    return nil // use container entrypoint
}

func (a *MyGameAdapter) Args(gs *gamesv1alpha1.GameServer) []string {
    return nil
}

func (a *MyGameAdapter) Probes(gs *gamesv1alpha1.GameServer) (*corev1.Probe, *corev1.Probe) {
    return nil, nil // no probes
}

func (a *MyGameAdapter) DataPath(gs *gamesv1alpha1.GameServer) string {
    return "/data"
}

func (a *MyGameAdapter) DefaultSecurityContext() *corev1.PodSecurityContext {
    return &corev1.PodSecurityContext{
        RunAsUser:  ptr.To(int64(1000)),
        RunAsGroup: ptr.To(int64(1000)),
        FSGroup:    ptr.To(int64(1000)),
    }
}

func init() {
    Register(&MyGameAdapter{})
}
```

### 2. Key methods

| Method | Purpose |
|--------|---------|
| `Name()` | Returns the `spec.game.type` value this adapter handles |
| `Env()` | Maps GameServer fields to container environment variables |
| `Command()` | Override the container entrypoint (return nil for default) |
| `Args()` | Override the container arguments (return nil for default) |
| `Probes()` | Return readiness and liveness probes (return nil to skip) |
| `DataPath()` | Container mount path for persistent data |
| `DefaultSecurityContext()` | Default pod-level security context |

### 3. Add the `game.type` validation

Update `internal/webhook/v1alpha1/gameserver_webhook.go` to add game type validation if needed.

### 4. Test

```bash
make test
```

### 5. Example GameServer

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServer
metadata:
  name: mygame-server
spec:
  game:
    type: mygame
    version: "1.0"
  runtime:
    image: mygame/server:latest
  storage:
    size: 5Gi
  network:
    ports:
      - name: game
        port: 8080
```

## Minecraft Adapter Details

The Minecraft adapter maps fields to [itzg/minecraft-server](https://github.com/itzg/docker-minecraft-server) environment variables:

| GameServer Field | Environment Variable |
|-----------------|---------------------|
| `game.type` + `game.profile` | `TYPE` (PAPER, FORGE, FABRIC, SPIGOT, BUKKIT, VANILLA) |
| `game.version` | `VERSION` |
| `server.maxPlayers` | `MAX_PLAYERS` |
| `server.motd` | `MOTD` |
| `server.difficulty` | `DIFFICULTY` |
| `server.gameMode` | `MODE` |
| `server.pvp` | `PVP` |
| `server.onlineMode` | `ONLINE_MODE` |