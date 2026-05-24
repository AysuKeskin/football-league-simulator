.PHONY: help run test test-v test-fresh test-docker vet build docker-up docker-down docker-logs migrate-up migrate-down migrate-version seed clean

# Database URL used by migrate / seed targets. These targets always run
# in throwaway containers that reach the compose-managed Postgres via the
# host gateway, so the host-side DSN is hardcoded here. Do NOT switch to
# ?= — that would let a sourced .env override this with the in-network
# `postgres` hostname, which throwaway containers cannot resolve.
MIGRATE_DATABASE_URL := postgres://fls:fls@host.docker.internal:5432/fls?sslmode=disable

# Default target shows help.
.DEFAULT_GOAL := help

## help: show available targets
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

## run: run the API locally with go run
run: ## run the API locally
	go run ./cmd/server

## test: run all unit tests (quiet)
test: ## run all unit tests
	go test ./...

## test-v: run all unit tests with per-test output
test-v: ## verbose test run
	go test -v ./...

## test-fresh: ignore the test cache (re-runs everything)
test-fresh: ## bypass test cache
	go test -count=1 ./...

## test-docker: run tests inside a Go container (no local Go required)
## Uses named volumes so module downloads + build cache persist across runs.
## First run is slow (deps downloaded); subsequent runs are fast.
test-docker: ## run tests in a Go container
	docker run --rm \
		-v "$(PWD)":/src \
		-v fls-gomod:/go/pkg/mod \
		-v fls-gocache:/root/.cache/go-build \
		-w /src \
		golang:1.25-alpine \
		go test -v ./...

## vet: run go vet across all packages
vet: ## run go vet
	go vet ./...

## build: compile the server binary into ./bin/server
build: ## compile the server binary
	@mkdir -p bin
	go build -o bin/server ./cmd/server

## docker-up: build and start the full stack (app + postgres)
docker-up: ## start app + postgres via docker compose
	docker compose up --build -d

## docker-down: stop and remove containers (volumes preserved)
docker-down: ## stop the stack
	docker compose down

## docker-logs: tail logs from all services
docker-logs: ## tail compose logs
	docker compose logs -f

## migrate-up: apply all pending database migrations
migrate-up: ## apply pending migrations
	docker run --rm \
		-v "$(PWD)/database/migrations":/migrations \
		--add-host=host.docker.internal:host-gateway \
		migrate/migrate \
		-path=/migrations \
		-database "$(MIGRATE_DATABASE_URL)" \
		up

## migrate-down: roll back the most recent migration
migrate-down: ## roll back one migration
	docker run --rm \
		-v "$(PWD)/database/migrations":/migrations \
		--add-host=host.docker.internal:host-gateway \
		migrate/migrate \
		-path=/migrations \
		-database "$(MIGRATE_DATABASE_URL)" \
		down 1

## migrate-version: print the current migration version
migrate-version: ## print current migration version
	docker run --rm \
		-v "$(PWD)/database/migrations":/migrations \
		--add-host=host.docker.internal:host-gateway \
		migrate/migrate \
		-path=/migrations \
		-database "$(MIGRATE_DATABASE_URL)" \
		version

## seed: load default teams into the database
seed: ## load default teams
	docker run --rm \
		-v "$(PWD)/database":/sql \
		--add-host=host.docker.internal:host-gateway \
		-e PGPASSWORD=fls \
		postgres:16-alpine \
		psql -h host.docker.internal -U fls -d fls -f /sql/seed.sql

## clean: remove build artifacts
clean: ## remove build artifacts
	rm -rf bin
