# Changelog

All notable changes to the GobeHost Operator are documented here.

## v0.4.0 - 2026-06-06

### Added

- **GameServerBackup CRD**: New CRD for scheduling and managing backups of GameServer and GameServerFleet data to S3-compatible storage.
  - Dedicated CRD decoupled from GameServer lifecycle
  - CronJob-based scheduled backups using rclone
  - Platform S3 defaults via ConfigMap/Secret in operator namespace
  - Per-backup storage overrides (endpoint, bucket, path, credentials)
  - Metadata inclusion (CRD YAML + referenced Secrets/ConfigMaps)
  - Backup-on-delete finalizer for pre-deletion backups
  - Configurable retention (oldest backups pruned first)
  - Short name: `gsb`

### Removed

- **BackupSpec from GameServer**: The `backup` field and `BackupSpec` struct have been removed from `GameServerSpec`. Backup configuration now uses the dedicated `GameServerBackup` CRD. The `BackupInProgress` phase is retained for coordination during backup-on-delete.

## v0.3.1 - 2026-06-03

### Added

- **RCON graceful shutdown countdown**: Fleet can warn players before an update by sending RCON commands to the Minecraft server. Configured via `spec.gracefulShutdown` with `enabled`, `countdownSeconds`, and `rconPort`. Password read from `RCON_PASSWORD` env var.
- **RCON client** (`internal/rcon/rcon.go`): Lightweight Minecraft RCON protocol implementation for sending console commands.
- **GracefulShutdownSpec**: New field on GameServerFleet spec.
- **CI path filters**: Lint, test, and e2e workflows skip on docs-only changes.

### Documentation

- Minecraft guide updated: in-place update flow, RCON graceful shutdown setup, link to [itzg container docs](https://docker-minecraft-server.readthedocs.io/).

## v0.3.0 - 2026-06-03

### Fixed

- **GameServer spec propagation**: `CreateOrUpdate` mutate functions were empty, meaning spec changes (image, env, resources, ports, security) were never applied to StatefulSet/Services after initial creation. Populated with proper desired-state sync.
- **Fleet rolling update preserves data**: Previously created a new GameServer with hash-suffixed name, which spawned a new StatefulSet with a new PVC — game data was lost during updates. Now updates the existing GameServer spec in-place: same StatefulSet, same PVC.
- **Helm webhooks.yaml YAML structure**: Fixed missing apiVersion/kind for ValidatingWebhookConfiguration, incorrect indentation on rules block, orphaned vgameserverfleet webhook, and wrong resource reference (gameservers → gameserverfleets).
- **E2E image loading**: Changed from `kind load docker-image` (broken on CI) to save-then-archive pattern for reliable cross-runtime loading.

### Removed

- **Standalone PVC creation**: The GS controller was creating a PVC (`{name}-data`) that was never mounted by the pod. The actual data PVC comes from the StatefulSet's VolumeClaimTemplate. Removed to avoid confusion.

### Changed

- **Fleet rolling update simplified**: Replaced the multi-phase state machine (waiting-for-ready → draining-old → complete) with direct in-place spec updates. `UpdatedGameServer` field and `UpdatePhaseAnnotation` are no longer used.
- **Refactored for lint**: Extracted `startSpecUpdate`, `completeUpdate`, `handleFailedGS`, `patchTemplateHash`, and `findCurrentGS` helpers from `handleSteadyState` to reduce cyclomatic complexity.
- **Helm chart published on GitHub Pages**: chart .tgz and index.yaml deployed alongside MkDocs docs site at `/charts/`.

### Added

- **Release workflow** (`.github/workflows/release.yml`): CI builds and pushes container image on `v*` tags.
- **`chart-sync-version` Makefile target**: Syncs chart `values.yaml` tag and `Chart.yaml` version/appVersion.
- **E2E lint fix**: Suppressed `errcheck` warnings on deferred cleanup calls in test utils.

## v0.2.0 - 2026-06-01

### Added

- **GameServerFleet CRD**: New CRD for managing the lifecycle of GameServers with:
  - Rolling update strategy (zero-downtime version upgrades)
  - Recreate strategy (for dev/staging environments)
  - Stable Service pattern (fleet-named Service updated at cutover)
  - Rollout history tracking (last 10 rollouts with versions, timestamps, results)
  - Fleet phases: Progressing, Available, Degraded, Failed
  - Webhook validation (replicas must equal 1, required fields)
  - Webhook defaulting (strategy defaults to RollingUpdate)
- **Fleet Controller**: Full reconciliation logic with:
  - GameServer creation from fleet template
  - Status synchronization (phase, readyReplicas, conditions)
  - Rolling update state machine (waiting-for-ready → draining-old → complete)
  - Recreate strategy support
  - Finalizer-based cleanup (removes GameServers and Service on deletion)
  - Template hash-based change detection
- **Fleet Service Builder**: Stable Service (`internal/reconciler/fleetservice.go`)
- **Fleet Webhook**: Defaulting and validation (`internal/webhook/v1alpha1/gameserverfleet_webhook.go`)
- **Tests**: 12 controller tests + 11 webhook tests for GameServerFleet
- **Helm chart**: Updated RBAC, webhooks, and CRD for GameServerFleet
- **Sample YAML**: `config/samples/games_v1alpha1_gameserverfleet.yaml`

## v0.1.0 - 2026-05-28

### Added

- **GameServer CRD**: Full spec for game server configuration
  - Game, Runtime, Storage, Network, Server, Backup, Scheduling, Security specs
  - Phases: Pending, Provisioning, Running, Stopping, Stopped, Failed
  - Status conditions with Ready tracking
  - LoadBalancer address reporting
- **GameServer Controller**: Reconciliation of StatefulSet, Services (headless + external), PVC
- **Minecraft Adapter**: itzg/minecraft-server integration with profile mapping
- **Webhooks**: Defaulting (security, ports, storage) and validation (required fields, immutability)
- **Helm Chart**: Manual chart with cert-manager integration
- **Tests**: 97.6% adapter coverage, 59.1% controller coverage, 100% webhook coverage