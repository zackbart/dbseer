.PHONY: dev build test test-integration lint fmt clean web-build web-dev web-install

BINARY := dbseer
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# `make dev` — run Vite and Go with hot-reload. Go server is dev-tagged so it
# reverse-proxies non-/api requests to Vite on :5173. Visit http://localhost:4983.
dev:
	@command -v air >/dev/null 2>&1 || { echo "installing air..."; go install github.com/cosmtrek/air@latest; }
	@(cd web && pnpm install --silent && pnpm dev) & air -c .air.toml

# `make build` — full production build: frontend then Go binary with embedded assets.
build: web-build
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/dbseer

web-install:
	cd web && pnpm install

web-build: web-install
	cd web && pnpm build

web-dev: web-install
	cd web && pnpm dev

test:
	go test ./...

test-integration:
	DBSEER_TEST_POSTGRES_DSN="$(DBSEER_TEST_POSTGRES_DSN)" go test ./internal/db -run Integration

lint:
	golangci-lint run ./...
	cd web && pnpm lint

fmt:
	gofmt -w .
	cd web && pnpm format

clean:
	rm -f $(BINARY)
	rm -rf web/node_modules .air-tmp
	# Reset the embed source to a placeholder so `go build ./...` still works
	# after `make clean`. Real builds regenerate this via `pnpm build`.
	rm -rf internal/ui/dist
	mkdir -p internal/ui/dist
	printf '<!doctype html><title>dbseer</title><p>run `make build`</p>\n' > internal/ui/dist/index.html
