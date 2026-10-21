GO ?= go

.PHONY: dev migrate-up migrate-down sqlc test test-int lint sec build css
dev:
	docker compose --env-file .env -f deploy/docker-compose.yml up --build
migrate-up:
	$(GO) run ./cmd/app migrate up
migrate-down:
	$(GO) run ./cmd/app migrate down
sqlc:
	$(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
test:
	$(GO) test ./...
test-int:
	$(GO) test -tags integration -count=1 ./internal/store ./internal/http
lint:
	$(GO) vet ./...
sec:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
	$(GO) run -modfile=tools/security/go.mod github.com/securego/gosec/v2/cmd/gosec -exclude-generated ./cmd/... ./internal/... ./db/... ./web/...
build:
	$(GO) build ./cmd/app
css:
	npm run css
