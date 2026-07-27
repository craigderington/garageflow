.PHONY: dev build api web migrate docker-up docker-down docker-logs test test-e2e e2e db-reset lint

dev:
	bash scripts/dev.sh

api:
	cd apps/api-go && go run ./cmd/server

web:
	cd apps/web-next && npm run dev

migrate:
	bash scripts/migrate.sh

build:
	cd apps/api-go && go build -o bin/server ./cmd/server
	cd apps/web-next && npm run build

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

db-reset:
	bash scripts/db-reset.sh

test-e2e:
	cd apps/web-next && npm run test:e2e

# Deterministic regression: reset DB to seed, then run the full suite.
e2e:
	bash scripts/e2e.sh

test:
	cd apps/api-go && go test ./...
	cd apps/web-next && npm run test:e2e

lint:
	cd apps/api-go && go vet ./...
	cd apps/web-next && npm run lint
