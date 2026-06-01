# Installation

## Prerequisites

- Kubernetes 1.27+
- Helm 3.8+ (recommended) or kubectl 1.27+
- Longhorn or other CSI storage provider (for persistent data)

## Install with Helm (Recommended)

```bash
helm repo add gobehost https://alienascension.github.io/gobehost-operator/charts
helm repo update
helm install gobehost-operator gobehost/gobehost-operator \
  --namespace gobehost-system \
  --create-namespace
```

## Install with Kustomize

```bash
kubectl apply -f https://raw.githubusercontent.com/AlienAscension/gobehost-operator/main/dist/install.yaml
```

## Install from Source

```bash
git clone https://github.com/AlienAscension/gobehost-operator.git
cd gobehost-operator
make deploy IMG=linusdb/gobehost:latest
```

## Verify Installation

```bash
kubectl get pods -n gobehost-system
kubectl get crd | grep gobehost
```

You should see the `gameservers` and `gameserverfleets` CRDs registered.

## Custom Image

To use a custom image registry:

```bash
helm install gobehost-operator gobehost/gobehost-operator \
  --namespace gobehost-system \
  --set image.repository=your-registry/gobehost \
  --set image.tag=v0.2.0
```

Or with Kustomize:

```bash
make deploy IMG=your-registry/gobehost:v0.2.0
```

## Uninstall

```bash
# Helm
helm uninstall gobehost-operator -n gobehost-system

# Kustomize
make undeploy
```

!!! warning "Data Preservation"
    Uninstalling the operator does **not** delete GameServer or GameServerFleet resources. Those must be deleted separately. PersistentVolumeClaims are retained unless explicitly deleted.