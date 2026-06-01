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

Wrap your Minecraft server in a fleet for rolling updates:

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServerFleet
metadata:
  name: minecraft-prod
spec:
  replicas: 1
  strategy:
    type: RollingUpdate
  template:
    spec:
      game:
        type: minecraft
        version: "26.1.2"
      runtime:
        image: itzg/minecraft-server:java25
      storage:
        size: 20Gi
      network:
        ports:
          - name: minecraft
            port: 25565
            protocol: TCP
        serviceType: LoadBalancer
```

Update the game version by editing `spec.template.spec.game.version`. The fleet controller handles the rolling update automatically.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Pod not starting | `kubectl describe pod <name>-0` |
| Phase stuck at Provisioning | `kubectl logs <name>-0 -c server` |
| No external IP | `kubectl get svc <name>` - LoadBalancer provisioning may take time |
| Adapter not found | Ensure `game.type` matches a registered adapter (currently: `minecraft`) |