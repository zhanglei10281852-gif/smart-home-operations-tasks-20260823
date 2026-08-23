# Smart Home Operations

Smart Home Operations is a Go backend for household onboarding, device lifecycle
management, telemetry, energy plans, automation execution, alerts, audit, and
reliable outbound delivery. PostgreSQL is the only production data store.

## Architecture

- `cmd/server`: configuration, migration, HTTP lifecycle, workers, and graceful shutdown.
- `internal/domain`: state rules, schedules, batch execution, health, and concurrency primitives.
- `internal/service`: authentication, device, telemetry, energy, automation, alert, and maintenance use cases.
- `internal/repo`: PostgreSQL repositories, serializable transactions, optimistic updates, outbox claims, and tenant ownership lookup.
- `internal/httpapi`: JSON API, request IDs, structured errors, authentication, tenant authorization, recovery, and body limits.
- `internal/worker`: cancellable automation and outbox processing with retries and leases.
- `internal/transport` and `internal/integration`: context-aware HTTP clients, retry policy, and circuit breaking.

The initial migration creates households, members, revocable sessions, devices,
capabilities, telemetry, energy plans, plan-device links, automations, actions,
runs, alerts, audit events, and outbox messages. Foreign keys and uniqueness
constraints protect cross-entity relationships. Device enrollment and household
onboarding are transactional; device transitions use optimistic versions;
automation and outbox claims use PostgreSQL row locking.

Set `OUTBOX_WEBHOOK_URL` to an absolute HTTPS endpoint to enable delivery of
durable device-command messages. Without it, the worker leaves messages queued
for a later configured process; it never marks an undelivered message as sent.

## Run

Go 1.26 is required. Copy values from `.env.example` into the process environment,
then start PostgreSQL and the server:

```sh
docker compose up -d postgres
DATABASE_URL=postgres://smart_home:smart_home@127.0.0.1:55432/smart_home?sslmode=disable GOTOOLCHAIN=local go run ./cmd/server
```

`GET /healthz` reports process liveness. `GET /readyz` verifies PostgreSQL before
reporting readiness. Migrations run at startup and are safe to repeat. The server
handles interrupt and termination signals, stops request intake, cancels workers,
and closes the database pool.

Create a household with its first owner through `POST /v1/households`. Login at
`POST /v1/households/{id}/sessions`; protected requests use
`Authorization: Bearer <session-id>`. Sessions expire, can be revoked through
`DELETE /v1/sessions/{id}`, and carry owner, operator, or viewer permissions.
Protected device, plan, telemetry, and automation routes also verify household
ownership of path resources.

## Verification

```sh
make test
make test-race
make vet
make build
docker build -t smart-home-operations:local .
```

The PostgreSQL integration test uses `DATABASE_URL` or the compose default above.
It does not skip when PostgreSQL is unavailable. It covers repeatable migrations,
transaction rollback, idempotency, outbox leases, and restart recovery.
