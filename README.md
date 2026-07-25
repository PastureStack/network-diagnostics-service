# Network Diagnostics Service

This repository provides the authenticated aggregation and bundle service used by PastureStack Network Diagnostics. It receives pseudonymous, bounded counter snapshots from one agent per managed host, applies independent validation and retention limits, and creates downloadable ZIP bundles for an authorized operator.

PastureStack is an independent community effort to preserve, audit, and modernize the Rancher 1.6 ecosystem. It is not affiliated with or endorsed by Rancher Labs or SUSE.

**Historical upstream reference:** [`rancher/network-diagnostics`](https://github.com/rancher/network-diagnostics). GitHub retains the fork relationship for provenance, but that upstream repository has no repository-level license. The public `main` branch, release source, and container image therefore contain only the new PastureStack implementation and do not redistribute the upstream source tree or Git history. See `LICENSING.md` and `ORIGIN.md`.

## Runtime

Start the service with a random deployment token:

```sh
docker run --rm \
  -p 8080:8080 \
  -e PASTURESTACK_DIAGNOSTICS_TOKEN='replace-with-a-random-secret-of-at-least-32-characters' \
  -v diagnostics-data:/var/lib/pasturestack-network-diagnostics \
  ghcr.io/pasturestack/network-diagnostics-service:v0.2.0
```

The container runs as numeric user `65532` from a shell-free image. Snapshot and bundle files are created with mode `0600`; storage directories use mode `0700`.

Authenticated endpoints require `Authorization: Bearer <token>`:

| Endpoint | Method | Purpose |
|---|---|---|
| `/healthz` | `GET` | Unauthenticated liveness and version |
| `/v1/snapshots` | `POST` | Accept one strictly validated agent snapshot |
| `/v1/summary` | `GET` | Return aggregate agent and snapshot counts |
| `/v1/bundles` | `POST` | Create a bounded ZIP bundle |
| `/v1/bundles` | `GET` | List available bundle identifiers |
| `/v1/bundles/{id}` | `GET` | Download a bundle |
| `/v1/bundles/{id}` | `DELETE` | Delete a bundle immediately |

The root page has English and Traditional Chinese variants selected through `Accept-Language`. It never embeds or requests the administrative token.

Runtime settings:

| Variable | Default | Constraint |
|---|---|---|
| `PASTURESTACK_DIAGNOSTICS_TOKEN` | none | Required; 32–256 characters |
| `PASTURESTACK_DIAGNOSTICS_ADDRESS` | `:8080` | HTTP listener |
| `PASTURESTACK_DIAGNOSTICS_DATA_DIR` | `/var/lib/pasturestack-network-diagnostics` | Dedicated persistent directory |
| `PASTURESTACK_DIAGNOSTICS_HISTORY_LENGTH` | `12` | 1–100 snapshots per agent |
| `PASTURESTACK_DIAGNOSTICS_MAX_AGENTS` | `256` | 1–1,024 agents |
| `PASTURESTACK_DIAGNOSTICS_RETENTION_HOURS` | `24` | 1–168 hours |

Snapshots are limited to 64 KiB and seven days of clock skew history. Bundles are limited to 64 MiB. Direct host names, addresses, routes, resolver values, process names, commands, workload content, logs, and credentials are outside the accepted schema.

## Audit planner

The default command remains the deterministic aggregation-policy planner. The deployable container starts the `serve` command explicitly.

## Build and test

```sh
go test ./...
go vet ./...
go build -trimpath -o bin/network-diagnostics-service ./cmd/network-diagnostics-service
docker build --build-arg VERSION=v0.2.0 -t network-diagnostics-service:v0.2.0 .
```

Run `sh scripts/validate.sh` on Unix-like systems or `pwsh -File scripts/validate.ps1` on Windows for formatting, repeated tests, vetting, module verification, and reproducible-build checks.

## Licensing

The current PastureStack implementation is licensed under Apache License 2.0. This grant applies only to material in the current PastureStack root commit; it does not grant or imply rights to the separately hosted historical upstream source. Go toolchain notices used for reproducible builds are included under `LICENSES/`. See `LICENSING.md` and `ORIGIN.md` for the exact boundary.
