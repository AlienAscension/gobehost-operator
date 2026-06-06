# GameServerBackup CRD Reference

## Overview

`GameServerBackup` manages scheduled and on-demand backups of `GameServer` and `GameServerFleet` data to S3-compatible storage. It creates and manages a `CronJob` that periodically copies PVC data and (optionally) CRD metadata to an S3 bucket using [rclone](https://rclone.org/).

Backups are decoupled from the `GameServer` lifecycle: you create a `GameServerBackup` resource that references a GameServer or GameServerFleet, and the operator handles the rest. Platform operators configure default S3 credentials; end users simply enable backups.

**API Group:** `games.gobehost.com`
**Version:** `v1alpha1`
**Scope:** Namespaced
**Short Names:** `gsb`

---

## Spec Reference

### GameServerBackupSpec

| Field | Type | Required | Description |
|---|---|---|---|
| `targetRef` | [TargetReference](#targetreference) | Yes | Reference to the GameServer or GameServerFleet to back up. |
| `schedule` | `string` | Yes | Cron schedule for backups (e.g. `0 */6 * * *`). |
| `retention` | `integer` | No | Number of backups to keep. Oldest are pruned first. **Default:** `5`. |
| `storage` | [BackupStorageSpec](#backupstoragespec) | No | S3 storage configuration. Uses platform defaults if omitted. |
| `includeMetadata` | `boolean` | No | Include CRD YAML and referenced Secrets/ConfigMaps in backup. **Default:** `true`. |
| `backupOnDelete` | `boolean` | No | Trigger an on-demand backup before the target is deleted. **Default:** `true`. |

### TargetReference

| Field | Type | Required | Description |
|---|---|---|---|
| `kind` | `string` | Yes | Type of target. Must be `GameServer` or `GameServerFleet`. |
| `name` | `string` | Yes | Name of the target resource in the same namespace. |

### BackupStorageSpec

| Field | Type | Required | Description |
|---|---|---|---|
| `endpoint` | `string` | No | S3-compatible endpoint URL. Uses platform default if empty. |
| `bucket` | `string` | No | S3 bucket name. Uses platform default if empty. |
| `path` | `string` | No | Path prefix within the bucket. Defaults to `<namespace>/<target-name>`. |
| `secretRef` | [BackupSecretRef](#backupsecretref) | No | Reference to a Secret with S3 credentials. Uses platform default if omitted. |

### BackupSecretRef

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `string` | Yes | Name of the Secret in the same namespace as the GameServerBackup. Must contain `S3_ACCESS_KEY` and `S3_SECRET_KEY` keys. |

---

## Status Reference

### GameServerBackupStatus

| Field | Type | Description |
|---|---|---|
| `lastBackupTime` | `Time` | Timestamp of the last completed backup attempt. |
| `lastBackupStatus` | `string` | `Success` or `Failed`. |
| `nextBackupTime` | `Time` | Estimated next backup time from the CronJob schedule. |
| `observedGeneration` | `integer` | The `metadata.generation` last processed by the controller. |
| `conditions` | [`[]Condition`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-meta-v1) | Standard status conditions. |

### Conditions

| Type | Description |
|---|---|
| `Ready` | `True` when the CronJob has been created and storage config is valid. |
| `LastBackupSucceeded` | `True` when the last backup Job completed successfully. |

### Condition Reasons

| Reason | Description |
|---|---|
| `CronJobCreated` | The backup CronJob is active. |
| `TargetNotFound` | The referenced GameServer or GameServerFleet does not exist. |
| `StorageConfigMissing` | No S3 storage configuration available (neither spec nor platform defaults). |
| `CronJobFailed` | Failed to create or update the backup CronJob. |
| `PVCNotReady` | The target's persistent volume claim is not bound. |
| `InvalidCredentials` | S3 credentials are invalid or missing. |
| `StorageUnavailable` | S3 bucket is not accessible. |

### Print Columns

| Name | Type | JSONPath |
|---|---|---|
| Schedule | string | `.spec.schedule` |
| LastBackup | date | `.status.lastBackupTime` |
| Status | string | `.status.lastBackupStatus` |
| Age | date | `.metadata.creationTimestamp` |

---

## Platform Configuration

The operator reads default S3 configuration from a ConfigMap and Secret in its own namespace (default: `gobehost-system`).

### ConfigMap: `gobehost-backup-config`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gobehost-backup-config
  namespace: gobehost-system
data:
  DEFAULT_S3_ENDPOINT: "https://s3.us-east-1.amazonaws.com"
  DEFAULT_S3_BUCKET: "gobehost-backups"
  DEFAULT_S3_PATH_PREFIX: "backups"
  DEFAULT_S3_SECRET_NAME: "gobehost-backup-s3-creds"
```

### Secret: `gobehost-backup-s3-creds`

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gobehost-backup-s3-creds
  namespace: gobehost-system
type: Opaque
stringData:
  S3_ACCESS_KEY: "<redacted>"
  S3_SECRET_KEY: "<redacted>"
```

### Resolution Order

For each `GameServerBackup`:

1. `spec.storage.secretRef.name` → if set, use that Secret (in the GameServerBackup's **namespace**, not the operator namespace)
2. Otherwise → read ConfigMap for `DEFAULT_S3_SECRET_NAME`, use that Secret from the operator namespace

Same for `endpoint`, `bucket`, `path`: spec override wins, else platform default.

---

## Example YAML

### Minimal (uses platform defaults)

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServerBackup
metadata:
  name: minecraft-survival-backup
  namespace: default
spec:
  targetRef:
    kind: GameServer
    name: minecraft-survival
  schedule: "0 */6 * * *"
```

### Full (custom S3 configuration)

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServerBackup
metadata:
  name: minecraft-survival-backup
  namespace: default
spec:
  targetRef:
    kind: GameServer
    name: minecraft-survival
  schedule: "0 */6 * * *"
  retention: 10
  storage:
    endpoint: "https://s3.us-east-1.amazonaws.com"
    bucket: "my-game-backups"
    path: "minecraft/survival"
    secretRef:
      name: my-s3-creds
  includeMetadata: true
  backupOnDelete: true
```

### Backing up a GameServerFleet

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServerBackup
metadata:
  name: my-fleet-backup
  namespace: default
spec:
  targetRef:
    kind: GameServerFleet
    name: my-fleet
  schedule: "0 0 * * *"
```

---

## Backup Contents Structure

```
s3://<bucket>/<path>/
├── 2026-06-06T12-00-00Z/
│   ├── data/                          # PVC contents
│   └── .gobehost-metadata/
│       ├── cr.yaml                    # GameServer/GameServerFleet CR
│       ├── secrets/
│       │   └── <secret-name>.yaml     # Referenced secrets
│       └── configmaps/
│           └── <cm-name>.yaml          # Referenced configmaps
├── 2026-06-06T18-00-00Z/
│   └── ...
```

When `includeMetadata` is `true`, the backup job exports the target CR's YAML and any referenced Secrets/ConfigMaps into `.gobehost-metadata/` alongside the PVC data.

---

## Lifecycle

### Creation

1. User creates a `GameServerBackup` referencing a GameServer or GameServerFleet
2. Controller resolves the target and its PVC
3. Controller reads platform S3 config (ConfigMap + Secret)
4. Controller creates a CronJob that runs rclone on the schedule
5. CronJob jobs mount the target PVC read-only and copy data to S3

### How rclone is configured

The controller configures rclone entirely through environment variables — no rclone config file or CLI flags are needed. The remote name is `s3_backup` (underscores, not hyphens — rclone env var convention).

| Env Var | Purpose |
|---|---|
| `RCLONE_CONFIG_S3_BACKUP_TYPE` | Always `s3` |
| `RCLONE_CONFIG_S3_BACKUP_PROVIDER` | Always `Other` (for MinIO and non-AWS S3) |
| `RCLONE_CONFIG_S3_BACKUP_ENDPOINT` | S3 endpoint URL (e.g. `http://192.168.0.34:9000`) |
| `RCLONE_CONFIG_S3_BACKUP_ACCESS_KEY_ID` | S3 access key (from Secret) |
| `RCLONE_CONFIG_S3_BACKUP_SECRET_ACCESS_KEY` | S3 secret key (from Secret) |
| `BACKUP_BUCKET` | S3 bucket name |
| `BACKUP_PATH` | Path prefix within the bucket |
| `BACKUP_RETENTION` | Number of backups to keep |
| `INCLUDE_METADATA` | Whether to include CRD YAML and Secrets |

The rclone command is static:

```
/usr/local/bin/rclone copy /data "s3_backup:$BACKUP_BUCKET/$BACKUP_PATH/$(date -u +%Y-%m-%dT%H-%M-%SZ)/"
```

??? tip "Why env vars instead of CLI flags?"
    CLI flags like `--s3-endpoint=http://host:9000` break when URLs contain colons (rclone parses them as remote delimiters). Environment variables avoid this entirely and follow the Kubernetes pattern of keeping the container command static while driving all configuration through env.

### Deletion (backupOnDelete)

When `backupOnDelete` is `true` (default):

1. Controller adds a finalizer on creation
2. On deletion, controller creates a one-shot Job before removing the finalizer
3. Once the backup Job completes, the finalizer is removed and the resource is garbage-collected

### Retention

The `retention` field specifies how many backups to keep. Oldest backups are pruned first. The CronJob container includes a pruning step that removes backups beyond the retention count.

---

## Related Resources

- **GameServer** — The game server being backed up
- **GameServerFleet** — A fleet being backed up (backs up the current GameServer)
- **CronJob** (`batch/v1`) — Created by the controller for scheduled backups
- **ConfigMap** / **Secret** — Platform S3 configuration in the operator namespace