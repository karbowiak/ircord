.PHONY: test check build frontend-check ergo-up ergo-down test-integration s3-up s3-down test-s3 test-browser-go
test:
	cd backend && go test -race ./...
check:
	cd backend && go vet ./...
	cd frontend && npm run api:check
build:
	mkdir -p .data/bin
	cd backend && go build -o ../.data/bin/ircord ./cmd/ircord
frontend-check:
	cd frontend && npm run api:check && npm run typecheck && npm test && npm run build

.PHONY: dev-backend
dev-backend:
	sh backend/dev.sh
ergo-up:
	sh infra/ergo/setup.sh
ergo-down:
	docker compose -f infra/ergo/compose.yaml down
test-integration: ergo-up
	cd backend && IRCORD_ERGO_ADDR=127.0.0.1:16667 go test -race -count=1 -timeout=240s ./internal/irc ./internal/api

s3-up:
	docker compose -f infra/s3/compose.yaml up -d --wait
s3-down:
	docker compose -f infra/s3/compose.yaml down
test-s3: s3-up
	cd backend && IRCORD_TEST_S3_ENDPOINT=127.0.0.1:19000 go test -race -count=1 ./internal/filehost
test-browser-go: build ergo-up frontend-check
	cd frontend && node scripts/verify-go.mjs

.PHONY: api-generate api-check
api-generate:
	cd frontend && npm run api:generate
api-check:
	cd frontend && npm run api:check
