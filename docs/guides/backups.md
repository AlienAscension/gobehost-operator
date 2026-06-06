# Backups

This guide covers setting up scheduled backups for GameServers and GameServerFleets using the `GameServerBackup` CRD.

## Prerequisites

- GobeHost Operator v0.4.0 or later
- An S3-compatible storage backend (MinIO, AWS S3, etc.)

## Platform Setup

The operator reads default S3 configuration from a ConfigMap and Secret in its own namespace.

### 1. Create the S3 credentials Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gobehost-backup-s3-creds
  namespace: gobehost-system
type: Opaque
stringData:
  S3_ACCESS_KEY: "your-access-key"
  S3_SECRET_KEY: "your-secret-key"
```

!!! warning "Secret key names must be `S3_ACCESS_KEY` and `S3_SECRET_KEY`"

### 2. Create the platform ConfigMap

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

Apply both:

```bash
kubectl apply -f config/samples/backup-platform-config.yaml
```

## Creating a Backup

### Minimal (uses platform defaults)

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServerBackup
metadata:
  name: minecraft-survival-backup
spec:
  targetRef:
    kind: GameServer
    name: minecraft-survival
  schedule: "0 */6 * * *"
```

### Custom S3 configuration

```yaml
apiVersion: games.gobehost.com/v1alpha1
kind: GameServerBackup
metadata:
  name: minecraft-survival-backup
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

## Checking backup status

```bash
kubectl get gameserverbackup
kubectl describe gameserverbackup minecraft-survival-backup
```

## How it works

1. The controller creates a CronJob that runs `rclone copy` on schedule
2. Each job mounts the target PVC read-only and copies data to S3
3. Backups are stored as `<bucket>/<path>/<timestamp>/`
4. When `backupOnDelete` is true (default), deleting the GameServerBackup triggers a final backup before removal

## Troubleshooting

### rclone errors

Check the backup job pod logs:

```bash
kubectl logs job/<backup-name>-backup-on-delete
```

### Common issues

| Error | Cause | Fix |
|---|---|---|
| `didn't find section in config file ("s3-backup")` | Old operator version using hyphenated remote name | Upgrade to v0.4.6+ |
| `endpoint 'http' was not a valid URI` | Old operator version using on-the-fly remote syntax | Upgrade to v0.4.5+ |
| `SignatureDoesNotMatch` | Wrong S3 credentials | Verify Secret keys are `S3_ACCESS_KEY` and `S3_SECRET_KEY` with correct values |
| `/usr/bin/rclone: not found` | Old operator version with wrong binary path | Upgrade to v0.4.3+ |

## Next steps

- [GameServerBackup CRD reference](crd/gameserverbackup.md) -- full API documentation
- [GameServer CRD reference](crd/gameserver.md) -- GameServer spec