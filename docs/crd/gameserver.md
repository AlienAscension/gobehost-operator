# GameServer CRD Reference

## Overview

The `GameServer` custom resource defines a dedicated game server instance managed by the gobehost-operator. It encapsulates the full lifecycle of a game server — from provisioning storage and networking through runtime configuration — into a single declarative Kubernetes resource. Backups are managed separately via the `GameServerBackup` CRD.

The operator reconciles a `GameServer` through the following lifecycle phases:

```
Pending → Provisioning → Running → Stopping → Stopped
                              ↓
                           Failed
                              ↓
                       BackupInProgress
```

| Phase | Description |
|---|---|
| **Pending** | The resource has been accepted but the controller has not yet started provisioning. |
| **Provisioning** | The controller is creating dependent resources (PVC, Deployment, Service, etc.). |
| **Running** | The game server pod is healthy and accepting connections. |
| **Stopping** | A graceful shutdown is in progress. |
| **Stopped** | The server has been shut down. |
| **Failed** | An unrecoverable error occurred during reconciliation. |
| **BackupInProgress** | A scheduled or manual backup is being taken. |

The `GameServer` resource uses a finalizer (`games.gobehost.com/finalizer`) to ensure clean deletion of dependent resources.

**API Group:** `games.gobehost.com`  
**Version:** `v1alpha1`  
**Scope:** Namespaced  
**Short Names:** `gs`, `gsv`

---

## Spec Reference

### GameServerSpec

| Field | Type | Required | Description |
|---|---|---|---|
| `game` | [GameSpec](#gamespec) | Yes | Game type and configuration. |
| `runtime` | [RuntimeSpec](#runtimespec) | Yes | Container runtime configuration. |
| `resources` | [ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core) | No | CPU/memory resource requests and limits. |
| `storage` | [StorageSpec](#storagespec) | Yes | Persistent volume configuration. |
| `network` | [NetworkSpec](#networkspec) | Yes | Network and port exposure configuration. |
| `server` | [ServerSpec](#serverspec) | No | Game-specific server settings. |

| `scheduling` | [SchedulingSpec](#schedulingspec) | No | Node placement constraints. |
| `security` | [SecuritySpec](#securityspec) | No | Pod security configuration. |

### GameSpec

| Field | Type | Required | Description |
|---|---|---|---|
| `type` | `string` | Yes | Game type (e.g. `minecraft`, `valheim`, `terraria`). |
| `version` | `string` | Yes | Game version (e.g. `1.21`, `0.218.15`). |
| `profile` | `string` | No | Optional game configuration profile name (e.g. `paper`, `vanilla`). |
| `mode` | `string` | No | Optional game mode (e.g. `survival`, `creative`). |

### RuntimeSpec

| Field | Type | Required | Description |
|---|---|---|---|
| `image` | `string` | Yes | Container image to run (e.g. `itzg/minecraft-server:java25`). |
| `imagePullPolicy` | `string` | No | Image pull policy. One of `Always`, `IfNotPresent`, `Never`. **Default:** `IfNotPresent`. |
| `command` | `[]string` | No | Override the container entrypoint. |
| `args` | `[]string` | No | Arguments to the entrypoint. |
| `env` | [`[]EnvVar`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core) | No | Environment variables. |
| `envFrom` | [`[]EnvFromSource`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envfromsource-v1-core) | No | Sources for environment variables (ConfigMaps/Secrets). |

### StorageSpec

| Field | Type | Required | Description |
|---|---|---|---|
| `size` | [Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#quantity-resource-autoscaling) | Yes | Requested storage size (e.g. `20Gi`). |
| `storageClass` | `string` | No | StorageClass name for the PVC. |
| `accessModes` | `[]string` | No | PVC access modes. **Default:** `[ReadWriteOnce]`. |

### NetworkSpec

| Field | Type | Required | Description |
|---|---|---|---|
| `ports` | [`[]PortSpec`](#portspec) | Yes | List of ports to expose. **Minimum:** 1 item. |
| `serviceType` | `string` | No | Service type for exposure. One of `ClusterIP`, `NodePort`, `LoadBalancer`. **Default:** `LoadBalancer`. |
| `hostname` | `string` | No | Optional DNS hostname for the server. |
| `annotations` | `map[string]string` | No | Annotations applied to the Service. |

### PortSpec

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `string` | Yes | Port name. |
| `port` | `integer` | Yes | Port number (1–65535). |
| `targetPort` | `integer` | No | Target port on the container. Defaults to `port`. |
| `protocol` | `string` | No | Network protocol. One of `TCP`, `UDP`, `SCTP`. **Default:** `TCP`. |

### ServerSpec

| Field | Type | Required | Description |
|---|---|---|---|
| `maxPlayers` | `integer` | No | Maximum number of players. |
| `motd` | `string` | No | Message of the day. |
| `levelName` | `string` | No | World/level name. |
| `whitelist` | `[]string` | No | List of allowed player names or UUIDs. |
| `ops` | `[]string` | No | List of operator player names or UUIDs. |
| `difficulty` | `string` | No | Game difficulty setting (e.g. `peaceful`, `easy`, `normal`, `hard`). |
| `gameMode` | `string` | No | Game mode (e.g. `survival`, `creative`, `adventure`). |
| `pvp` | `boolean` | No | Enable player-vs-player combat. |
| `onlineMode` | `boolean` | No | Enable online authentication. |

### SchedulingSpec

| Field | Type | Required | Description |
|---|---|---|---|
| `nodeSelector` | `map[string]string` | No | Node labels for pod placement. |
| `affinity` | [Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core) | No | Node/pod affinity rules. |
| `tolerations` | [`[]Toleration`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core) | No | Node tolerations. |

### SecuritySpec

| Field | Type | Required | Description |
|---|---|---|---|
| `runAsNonRoot` | `boolean` | No | Enforce running as non-root. **Default:** `true`. |
| `runAsUser` | `integer` | No | UID for the container process. **Default:** `1000`. |
| `runAsGroup` | `integer` | No | GID for the container process. **Default:** `1000`. |
| `fsGroup` | `integer` | No | GID for mounted volumes. **Default:** `1000`. |
| `readOnlyRootFilesystem` | `boolean` | No | Enforce a read-only root filesystem. |
| `dropAllCapabilities` | `boolean` | No | Drop all Linux capabilities. **Default:** `true`. |
| `seccompProfile` | `string` | No | Seccomp profile type. One of `RuntimeDefault`, `Unconfined`, `Localhost`. **Default:** `RuntimeDefault`. |

---

## Status Reference

### GameServerStatus

| Field | Type | Description |
|---|---|---|
| `phase` | `string` | Current lifecycle phase. See [Phases](#phases). |
| `ready` | `boolean` | `true` when the server is operational and accepting connections. |
| `conditions` | [`[]Condition`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-meta-v1) | Standard status conditions (type map, keyed by `type`). |
| `address` | `string` | IP address or hostname of the server. |
| `endpoint` | `string` | Full connection endpoint (address:port). |
| `ports` | [`[]PortInfo`](#portinfo) | List of exposed port information. |
| `playerCount` | `integer` | Current number of connected players. |
| `observedGeneration` | `integer` | The `metadata.generation` last processed by the controller. |

### PortInfo

| Field | Type | Description |
|---|---|---|
| `name` | `string` | Port name. |
| `port` | `integer` | Port number. |
| `protocol` | `string` | Network protocol. |

### Print Columns

The following columns are displayed in `kubectl get gameserver` output:

| Name | Type | JSONPath |
|---|---|---|
| Game | string | `.spec.game.type` |
| Version | string | `.spec.game.version` |
| Phase | string | `.status.phase` |
| Ready | boolean | `.status.ready` |
| Address | string | `.status.address` |
| Age | date | `.metadata.creationTimestamp` |

---

## Phases

### Pending

Initial state after creation. The controller has accepted the resource but has not yet started provisioning.

### Provisioning

The controller is creating dependent resources: PersistentVolumeClaim, Deployment, Service, and any other required infrastructure.

### Running

The game server pod is healthy, the Service is available, and the server is accepting player connections. `status.ready` is set to `true`.

### Stopping

A graceful shutdown has been triggered (e.g. deletion). The controller runs_finalizer logic to clean up resources.

### Stopped

The server has been fully shut down. All pods have terminated.

### Failed

An unrecoverable error occurred during reconciliation. Check `status.conditions` for details.

### BackupInProgress

A backup is currently being taken. The controller pauses reconciliation of other changes until the backup completes.

---

## Example YAML

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
    imagePullPolicy: IfNotPresent
    env:
      - name: PAPER_BUILD
        value: "66"
  resources:
    requests:
      cpu: 500m
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
    motd: "GobeHost Minecraft Server"
    difficulty: normal
    gameMode: survival
    pvp: false
    onlineMode: true
  security:
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
    seccompProfile: RuntimeDefault
```

---

## Webhook Defaults and Validations

### Defaulting Webhook

The mutating webhook sets the following defaults on create and update:

| Field | Default |
|---|---|
| `spec.runtime.imagePullPolicy` | `IfNotPresent` |
| `spec.storage.accessModes` | `[ReadWriteOnce]` |
| `spec.network.serviceType` | `LoadBalancer` |
| `spec.network.ports[].protocol` | `TCP` |
| `spec.network.ports[].targetPort` | Same as `port` |
| `spec.security.runAsNonRoot` | `true` |
| `spec.security.runAsUser` | `1000` |
| `spec.security.runAsGroup` | `1000` |
| `spec.security.fsGroup` | `1000` |
| `spec.security.seccompProfile` | `RuntimeDefault` |
| `spec.security.dropAllCapabilities` | `true` |

If `spec.security` is omitted entirely, the webhook creates it with all security defaults applied.

### Validation Webhook

The validating webhook enforces the following rules:

**On Create and Update:**

| Rule | Field | Message |
|---|---|---|
| Required | `spec.game.type` | Game type is required. |
| Required | `spec.game.version` | Game version is required. |
| Required | `spec.runtime.image` | Runtime image is required. |
| Required | `spec.network.ports` | At least one port is required. |
| Required | `spec.storage.size` | Storage size is required. |
| Range | `spec.network.ports[].port` | Must be between 1 and 65535. |
| Range | `spec.network.ports[].targetPort` | Must be between 1 and 65535 (when set). |

**On Update (immutable fields):**

| Field | Message |
|---|---|
| `spec.game.type` | Game type is immutable. Changing it on an existing GameServer is forbidden. |