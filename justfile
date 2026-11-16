# Layr Command Runner (justfile)

# Default recipe: list available recipes
default:
    @just --list

# Start both Layr server and Console UI development server concurrently
dev: build-dev-server
    @bash scripts/dev

# Start only the Layr server (monolith start by default, or specific services)
dev-server *ARGS: build-dev-server
    @exec ./dev-server up {{ ARGS }}

# Build the Layr server binary for development
build-dev-server:
    @go build -o ./dev-server ./cmd/layr

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
