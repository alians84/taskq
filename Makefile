GOLANGCI := go tool -modfile=tools/go.mod golangci-lint

.PHONY: lint test test-integration up down logs

lint:
	$(GOLANGCI) run ./...

test:
	go test -race ./...

test-integration:
	go test -race -tags=integration ./...

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f
