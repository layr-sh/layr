# Layr Command Runner (justfile)

# Default recipe: list available recipes
default:
    @just --list

# Start both Layr server and Console UI development server concurrently
dev:
    @bash -c 'trap "kill 0" INT TERM EXIT; just dev-server & just dev-console-ui & wait'

# Start only the Layr server (monolith start)
dev-server:
    go run ./cmd/layr up

# Start only the Console UI dev server with Hot Module Reloading (HMR)
dev-console-ui:
    pnpm --dir ui/console run dev

# Build the Console UI SPA bundle and embed it into the console service
build-console:
    pnpm --dir ui/console run build
    cp -r ui/console/dist/* console/dist/

# Build complete single-binary monolith with embedded production Console UI
build: build-console
    go build -o layr ./cmd/layr
