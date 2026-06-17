.PHONY: all build test vet fmt run dev frontend clean

BINARY := console-web
FRONTEND_OUT := frontend/out

all: build

# Force a clean rebuild of the static Next.js export (installs deps).
frontend:
	cd frontend && npm ci && npm run build

# Auto-build the export only if it is missing — go:embed needs it to exist
# at compile time. Run `make frontend` to force a rebuild after UI changes.
$(FRONTEND_OUT):
	cd frontend && npm ci && npm run build

# Build the single binary (depends on the embedded frontend export).
build: $(FRONTEND_OUT)
	go build -o $(BINARY) .

test: $(FRONTEND_OUT)
	go test -race ./...

vet: $(FRONTEND_OUT)
	go vet ./...

fmt:
	gofmt -w $$(git ls-files '*.go')

run: build
	./$(BINARY)

# Next.js dev server on :3000, proxying /api and /ws to a backend on :8080.
# Run the Go server separately (e.g. `go run .`) for the API/WS/PTY backend.
dev:
	cd frontend && npm run dev

clean:
	rm -f $(BINARY)
	rm -rf $(FRONTEND_OUT) frontend/.next
