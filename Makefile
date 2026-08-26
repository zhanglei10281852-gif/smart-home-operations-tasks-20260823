.PHONY: build test test-race vet run postgres-up postgres-down

DATABASE_URL ?= postgres://smart_home:smart_home@127.0.0.1:55432/smart_home?sslmode=disable

build:
	GOTOOLCHAIN=local go build ./...

test:
	DATABASE_URL="$(DATABASE_URL)" GOTOOLCHAIN=local go test ./... -count=1

test-race:
	DATABASE_URL="$(DATABASE_URL)" GOTOOLCHAIN=local go test -race ./... -count=1

vet:
	GOTOOLCHAIN=local go vet ./...

run:
	DATABASE_URL="$(DATABASE_URL)" GOTOOLCHAIN=local go run ./cmd/server

postgres-up:
	docker compose up -d postgres

postgres-down:
	docker compose down
