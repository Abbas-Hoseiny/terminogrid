# TerminoGrid Go Backend

Small Go server exposing the minimal API expected by `terminogrid/index.html`:

- `GET /api/containers`
- `POST /api/containers/{id}/start`
- `POST /api/containers/{id}/stop`
- `WS  /api/containers/{id}/exec?cols&rows&term`

When a Docker daemon is available (`DOCKER_HOST` or the Unix socket), the server uses the real Docker API for list/start/stop and an interactive exec bridge. Otherwise it falls back to a tiny in‑memory list for demo.

The exec bridge launches an interactive login shell (`/bin/bash -li`, fallback `/bin/sh -i`) and sets `TERM=xterm-256color` for proper terminal behavior.

## Develop

Requires Go 1.21+

```
cd backend
go mod tidy
go run ./cmd/server  # listens on :8080
```

Serve the UI via any static server and connect with `?api=http://localhost:8080`.

## WS/Resize protocol
- Client sends raw input plus JSON resize messages: `{ "type":"resize", "cols":N, "rows":N }`.
- Server streams output as binary or text; the frontend handles both.

## Production notes
- CORS is open for development; restrict behind a reverse proxy if required.
