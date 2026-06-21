.PHONY: generate test test-integration test-cover lint docs schema

generate:
	go generate ./...

test:
	go test -race -count=1 ./internal/... ./cmd/...

test-integration:
	go test -race -count=1 -tags integration -timeout 5m ./tests/...

test-cover:
	go test -count=1 -coverprofile=coverage.txt -covermode=atomic \
	    ./internal/service/... ./internal/storage/... \
	    ./internal/handler/... ./internal/middleware/...
	go tool cover -func=coverage.txt

lint:
	golangci-lint run --timeout=5m

# Render OpenAPI spec in browser (requires Node.js).
# Alternative: paste api/v1/openapi.yaml into https://editor.swagger.io
docs:
	npx --yes @redocly/cli preview-docs api/v1/openapi.yaml

# Regenerate the Mermaid ER diagram in README.md from migrations/*.up.sql.
# Run after adding a new migration.
schema:
	go run ./scripts/genschema
