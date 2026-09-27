.PHONY: run migrate fmt lint
run:
	go run ./cmd/bot

migrate:
	@set -eu; \
	: "$${POSTGRES_HOST:?POSTGRES_HOST is required}"; \
	: "$${POSTGRES_PORT:?POSTGRES_PORT is required}"; \
	: "$${POSTGRES_DATABASE:?POSTGRES_DATABASE is required}"; \
	: "$${POSTGRES_USER:?POSTGRES_USER is required}"; \
	: "$${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"; \
	: "$${POSTGRES_SSLMODE:?POSTGRES_SSLMODE is required}"; \
	export PGHOST="$$POSTGRES_HOST" PGPORT="$$POSTGRES_PORT" PGDATABASE="$$POSTGRES_DATABASE" PGUSER="$$POSTGRES_USER" PGPASSWORD="$$POSTGRES_PASSWORD" PGSSLMODE="$$POSTGRES_SSLMODE"; \
	GOOSE_DBSTRING="host=$$PGHOST port=$$PGPORT dbname=$$PGDATABASE user=$$PGUSER sslmode=$$PGSSLMODE"; \
	go run github.com/pressly/goose/v3/cmd/goose@v3.24.1 -dir migrations postgres "$$GOOSE_DBSTRING" up

fmt:
	@if command -v goimports >/dev/null 2>&1; then goimports -w $$(find cmd internal -name '*.go'); elif command -v golangci-lint >/dev/null 2>&1; then golangci-lint fmt; else gofmt -w $$(find cmd internal -name '*.go'); fi

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run; else go vet ./...; fi
