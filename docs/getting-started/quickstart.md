# Quickstart

Deploy a Minecraft server in under 5 minutes.

## 1. Apply the GameServer CRD

If you haven't installed the operator yet, follow the [Installation](installation.md) guide.

## 2. Create a GameServer

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServer
metadata:
  name: minecraft-survival
  namespace: default
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

```bash
kubectl apply -f minecraft.yaml
```

## 3. Check Status

```bash
kubectl get gameserver minecraft-survival -o wide
```

Wait until `PHASE` shows `Running` and `READY` shows `true`.

## 4. Connect

Once the `EXTERNAL-IP` is assigned:

```bash
kubectl get svc minecraft-survival
```

Connect with your Minecraft client to the external IP on port 25565.

## 5. Create a Fleet (Optional)

For lifecycle management with rolling updates:

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServerFleet
metadata:
  name: minecraft-prod
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
      storage:
        size: 10Gi
      network:
        ports:
          - name: minecraft
            port: 25565
            protocol: TCP
        serviceType: LoadBalancer
```

The fleet creates a GameServer and a stable Service named after the fleet. Rolling updates are applied by changing `spec.template.spec.game.version`.

```bash
kubectl apply -f fleet.yaml
kubectl get gameserverfleet minecraft-prod -o wide
```

## Next Steps

- [GameServer CRD Reference](../crd/gameserver.md)
- [GameServerFleet CRD Reference](../crd/gameserverfleet.md)
- [Deploying Minecraft Guide](../guides/minecraft.md)