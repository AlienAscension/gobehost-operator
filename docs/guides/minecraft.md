# Deploying Minecraft

This guide walks through deploying a production-grade Minecraft server with the GobeHost operator, including persistent storage, automatic backups, and Traefik TCP routing.

## Prerequisites

- GobeHost operator installed (see [Installation](../getting-started/installation.md))
- Longhorn or equivalent CSI storage provisioner
- Traefik with TCP entrypoint configured (for IngressRouteTCP)

## Basic Minecraft Server

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServer
metadata:
  name: minecraft-survival
spec:
  game:
    type: minecraft
    version: "26.1.2"
  runtime:
    image: itzg/minecraft-server:java25
  storage:
    size: 20Gi
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

## Server Implementations (`profile`)

The `profile` field selects **which server implementation** to run. These are mutually exclusive — pick one per GameServer. The operator maps `profile` to the `TYPE` environment variable consumed by the [itzg/minecraft-server](https://github.com/itzg/docker-minecraft-server) Docker image, which downloads and runs the corresponding server jar.

| `profile` value | `TYPE` env var | Server | Description |
|---|---|---|---|
| _(empty)_ | `VANILLA` | Mojang Vanilla | Official unmodded server |
| `paper` | `PAPER` | [PaperMC](https://papermc.io/) | High-performance fork of Spigot; best for most production servers |
| `spigot` | `SPIGOT` | [Spigot](https://www.spigotmc.org/) | Moddable server API with plugin support |
| `bukkit` | `BUKKIT` | [Bukkit](https://dev.bukkit.org/) | Original plugin API (predecessor of Spigot) |
| `forge` | `FORGE` | [Minecraft Forge](https://minecraftforge.net/) | Mod loader for deep Java mods |
| `fabric` | `FABRIC` | [Fabric](https://fabricmc.net/) | Lightweight mod loader |

The lineage is Bukkit → Spigot → Paper (each forked from the previous). Paper, Spigot, and Bukkit support **plugins** (Bukkit/Spigot plugin API). Forge and Fabric support **mods** (deeper game modifications that add blocks, items, dimensions, etc.).

## Paper Minecraft with Custom Build

Use the `profile` field to select a server implementation and `env` for additional configuration:

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServer
metadata:
  name: minecraft-paper
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
    motd: "Welcome to our server"
    difficulty: normal
    gameMode: survival
    pvp: false
    onlineMode: true
```

## Traefik IngressRouteTCP Integration

For TCP routing through Traefik instead of LoadBalancer:

```yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRouteTCP
metadata:
  name: minecraft
  namespace: default
spec:
  entryPoints:
    - minecraft
  routes:
    - match: HostSNI(`*`)
      services:
        - name: minecraft-survival
          port: 25565
```

This requires a Traefik `minecraft` entrypoint configured in your Traefik Helm values:

```yaml
ports:
  minecraft:
    port: 25565
    exposedPort: 25565
    expose:
      default: true
    protocol: TCP
```

## Using GameServerFleet for Lifecycle Management

Wrap your Minecraft server in a fleet for managed updates:

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServerFleet
metadata:
  name: minecraft-prod
spec:
  replicas: 1
  strategy:
    type: RollingUpdate
  gracefulShutdown:
    enabled: true
    countdownSeconds: 5
    rconPort: 25575
  template:
    spec:
      game:
        type: minecraft
        version: "26.1.2"
      runtime:
        image: itzg/minecraft-server:java25
        env:
          - name: EULA
            value: "TRUE"
          - name: RCON_PASSWORD
            value: "your-secure-password"
      storage:
        size: 20Gi
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

### How Updates Work

Editing `spec.template` triggers an in-place update:

1. Fleet detects template hash changed
2. If `gracefulShutdown.enabled`, sends RCON countdown to players (`say Server restarting in 5...`)
3. Fleet updates the existing GameServer spec
4. GS controller propagates changes to the StatefulSet
5. Pod restarts with new configuration — player data, world files, and plugins remain intact on the existing PVC

### Graceful Shutdown via RCON

To warn players before an update, enable `gracefulShutdown`:

```yaml
spec:
  gracefulShutdown:
    enabled: true
    countdownSeconds: 10   # 10-second warning before restart
    rconPort: 25575        # default Minecraft RCON port
```

The operator:
1. Connects to `<gs-name>-headless.<namespace>.svc.cluster.local:25575`
2. Authenticates with `RCON_PASSWORD` from the runtime env
3. Sends: `say Server restarting for update in 10 seconds`
4. Counts down: `say Server restarting in 9...`, `8...`, etc.
5. Proceeds with the update

If RCON fails (wrong password, server not ready), the update proceeds anyway — the countdown is best-effort.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Pod not starting | `kubectl describe pod <name>-0` |
| Phase stuck at Provisioning | `kubectl logs <name>-0 -c server` |
| No external IP | `kubectl get svc <name>` - LoadBalancer provisioning may take time |
| Adapter not found | Ensure `game.type` matches a registered adapter (currently: `minecraft`) |

## Further Reading

- [itzg/docker-minecraft-server documentation](https://docker-minecraft-server.readthedocs.io/) — environment variables, server types, mods, plugins, and all container configuration options