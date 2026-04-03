# Contributing

## Prerequisites

- **Go** 1.26 or later — [go.dev/dl](https://go.dev/dl/)
- **Node.js** v18 or later — [nodejs.org](https://nodejs.org)

Verify your installation:

```sh
go version    # should print go1.26 or higher
node --version # should print v18.x.x or higher
npm --version
```

## Running for development

Both the server and the frontend dev server must be running at the same time.

**Terminal 1 — Go server:**

```sh
cd server
go run .
```

The server listens on `http://localhost:8080`.

**Terminal 2 — Frontend:**

```sh
cd web
npm install
npm run dev
```

Then open [http://localhost:5173](http://localhost:5173) in your browser.

The Vite dev server proxies `/api` and `/ws` requests to the Go server automatically, so no CORS configuration is needed during development.

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT`   | `8080`  | TCP port the server listens on |

## Building for production

The production build embeds the frontend into the Go binary so a single executable serves both the API and the SPA.

**With Make (local build):**

```sh
make build
```

This runs `npm ci && npm run build` in `web/`, copies `web/dist/` into the Go embed directory, and compiles `server/writerace`.

**With Docker:**

```sh
make docker-build
```

This builds a multi-stage Docker image (`writerace`) that includes the frontend. The build context is the repository root.

To run the Docker image:

```sh
docker run -p 8080:8080 writerace
```

## Testing

**Unit tests** (no external dependencies, runs quickly):

    make unit-go

**Integration tests** (starts a real HTTP server in-process, tests WebSocket flows):

    make integration-test

**All tests:**

    make test-all
