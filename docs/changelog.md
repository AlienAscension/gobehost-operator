# Changelog

All notable changes to the GobeHost Operator are documented here.

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