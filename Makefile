.PHONY: build test seed migrate up down bench fmt vet clean

GO ?= go
APP := github.com/soma-genesis/scenario-2-pricing-perf

build:
	$(GO) build ./...

test:
	$(GO) test ./tests/public/...

bench:
	$(GO) run ./cmd/benchmark -profile small -rounds 100

seed:
	$(GO) run ./cmd/seed -out testdata/fixtures -profile medium

migrate:
	$(GO) run ./cmd/migrate -dir migrations

up:
	docker compose up --build -d

down:
	docker compose down -v

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

clean:
	rm -rf testdata/fixtures/*.json
