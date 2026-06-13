.PHONY: browser-smoke build dev frontend-build frontend-typecheck own-db smoke test

GO_ADDR ?= 127.0.0.1:8081
DB ?= tala.db
SMOKE_URL ?= http://$(GO_ADDR)
OWN_DB_ADDR ?= 127.0.0.1:8081
OWN_DB ?= tala.db

dev:
	go run ./cmd/tala -addr $(GO_ADDR) -db $(DB)

own-db:
	go run ./cmd/tala -addr $(OWN_DB_ADDR) -db $(OWN_DB)

frontend-build:
	bun run build

frontend-typecheck:
	bun run typecheck

test:
	go test ./...
	bun run typecheck

build:
	bun run build
	go build ./cmd/tala

smoke:
	scripts/smoke.sh $(SMOKE_URL)

browser-smoke:
	scripts/browser-smoke.sh $(SMOKE_URL)
