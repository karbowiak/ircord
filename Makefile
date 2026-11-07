ifeq ($(firstword $(MAKECMDGOALS)),cli)
# Treat goals after cli as client arguments, including names of normal targets.
CLI_COMMAND := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
.DEFAULT:
	@:
else
.PHONY: test check build cli frontend-check ergo-up ergo-down test-integration s3-up s3-down test-s3 test-browser-go
test:
	cd backend && go test -race ./...
	cd cli && go test -race ./...
check: api-check
	cd backend && go vet ./...
	cd cli && go vet ./...
build:
	mkdir -p .data/bin
	cd backend && go build -o ../.data/bin/ircord ./cmd/ircord
	cd cli && go build -o ../.data/bin/ircordctl ./cmd/ircordctl
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

.PHONY: test-read-markers
test-read-markers: ergo-up
	cd frontend && npm run build
	cd backend && IRCORD_ERGO_ADDR=127.0.0.1:16667 IRCORD_READ_MARKER_BROWSER=true go test -race -count=1 -timeout=180s -run '^TestErgoReadMarkerDevices$$' -v ./internal/api

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
	sh cli/generate.sh
api-check:
	cd frontend && npm run api:check
	sh cli/generate.sh --check
endif

.PHONY: cli
cli:
	@mkdir -p .data/bin
	@cd cli && go build -o ../.data/bin/ircordctl ./cmd/ircordctl
	@if [ -z "$${IRCORD_CLI_CONFIG:-}" ] && [ -f .data/admin-client.json ]; then \
		export IRCORD_CLI_CONFIG="$(CURDIR)/.data/admin-client.json"; \
	fi; \
	exec .data/bin/ircordctl $(CLI_COMMAND) $(ARGS)
