# Smart Home Operations

Smart Home Operations is a production-shaped Go backend for household device
enrollment, telemetry, energy planning, automation execution, alerts, and audit.
The service uses PostgreSQL for all durable state and exposes a small HTTP API.

Run PostgreSQL with `docker compose up -d postgres`, then start the service with
`DATABASE_URL=postgres://smart_home:smart_home@localhost:55432/smart_home?sslmode=disable go run ./cmd/server`.
Migrations run on startup and are safe to repeat.
