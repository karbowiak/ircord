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
	cd frontend && bun run api:check && bun run typecheck && bun run test && bun run build

.PHONY: backend frontend dev-backend
backend:
	@mkdir -p .data/bin
	cd backend && go build -o ../.data/bin/ircord ./cmd/ircord
	@set -eu; \
	if [ -f backend/.env ]; then set -a; . ./backend/.env; set +a; fi; \
	export IRCORD_DATA_DIR="$${IRCORD_DATA_DIR:-$(CURDIR)/.data/live}"; \
	export IRCORD_LISTEN="$${IRCORD_LISTEN:-127.0.0.1:8080}"; \
	export IRCORD_PUBLIC_URL="$${IRCORD_PUBLIC_URL:-http://127.0.0.1:3000}"; \
	export IRCORD_TRUSTED_PROXIES="$${IRCORD_TRUSTED_PROXIES:-127.0.0.1/32,::1/128}"; \
	export IRCORD_LINK_PREVIEWS="$${IRCORD_LINK_PREVIEWS:-true}"; \
	if [ -z "$${IRCORD_REGISTRATION_KEY:-}" ] && [ -z "$${IRCORD_REGISTRATION_KEY_FILE:-}" ] && [ -f .data/live/registration-key ]; then \
		export IRCORD_REGISTRATION_KEY_FILE="$(CURDIR)/.data/live/registration-key" IRCORD_REQUIRE_REGISTRATION_KEY=true; \
	fi; \
	if [ -z "$${IRCORD_NETWORK_PASSWORDS:-}" ] && [ -z "$${IRCORD_NETWORK_PASSWORDS_FILE:-}" ] && [ -f .data/live/network-passwords.json ]; then \
		export IRCORD_NETWORK_PASSWORDS_FILE="$(CURDIR)/.data/live/network-passwords.json"; \
	fi; \
	exec .data/bin/ircord
frontend:
	cd frontend && bun run dev
dev-backend: backend
ergo-up:
	sh infra/ergo/setup.sh
ergo-down:
	docker compose -f infra/ergo/compose.yaml down
test-integration: ergo-up
	cd backend && IRCORD_ERGO_ADDR=127.0.0.1:16667 go test -race -count=1 -timeout=240s ./internal/irc ./internal/api

.PHONY: test-read-markers
test-read-markers: ergo-up
	cd frontend && bun run build
	cd backend && IRCORD_ERGO_ADDR=127.0.0.1:16667 IRCORD_READ_MARKER_BROWSER=true go test -race -count=1 -timeout=180s -run '^TestErgoReadMarkerDevices$$' -v ./internal/api

s3-up:
	docker compose -f infra/s3/compose.yaml up -d --wait
s3-down:
	docker compose -f infra/s3/compose.yaml down
test-s3: s3-up
	cd backend && IRCORD_TEST_S3_ENDPOINT=127.0.0.1:19000 go test -race -count=1 ./internal/filehost
test-browser-go: build ergo-up frontend-check
	cd frontend && bun scripts/verify-go.mjs

.PHONY: api-generate api-check
api-generate:
	cd frontend && bun run api:generate
	sh cli/generate.sh
api-check:
	cd frontend && bun run api:check
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
