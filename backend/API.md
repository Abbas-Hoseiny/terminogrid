# TerminoGrid Backend – API Spec

Goal: Minimal backend used by `terminogrid/index.html`. Focus: list, start/stop, interactive shell via WebSocket.

## Base URL & Modes

- The UI discovers the API base via `?api=` query or same‑origin.
- Offline/Demo: If opened via `file://` or no HTTP origin, the UI uses demo data.

## Endpoints

### GET `/api/containers`

- Response can be either `[{...}]` or `{ containers: [{...}] }`.
- Accepted fields (Docker-compatible):

  - `id | ID`: string
  - `name | Names`: string (leading `/` is stripped by UI)
  - `image | Image`: string
  - `status | State`: `running|exited|paused|restarting|created|unknown`
  - `labels | Labels`: map
  - `ports | Ports`: either
    - array entries `{PublicPort, PrivatePort, Type}` (or `{public, private, type}`)
    - or a `NetworkSettings.Ports`-like map: `{ "80/tcp": [{"HostIp":"0.0.0.0","HostPort":"8080"}] }`

- Sorting/filtering is done in the UI.
- Status: `200` on success; errors with meaningful text/JSON.

### POST `/api/containers/{id}/start`

- Starts a container; short or long ID allowed.
- Status: `204` (preferred) or `200`; errors: `404`, `409`, `500`.

### POST `/api/containers/{id}/stop`

- Stops a container.
- Status: same as start.

### WS `/api/containers/{id}/exec`

- Query: `cols`, `rows`, `term` (e.g., `xterm-256color`).
- Creates interactive TTY (similar to `docker exec -it <id> /bin/bash`).
- Terminal session runs with enhanced color support:

  - `TTY=true` for proper terminal emulation
  - Environment variables: `TERM=xterm-256color`, `COLORTERM=truecolor`, `CLICOLOR_FORCE=1`, `FORCE_COLOR=1`
  - Automatic color aliases for `ls` and `grep` commands
  - APT color configuration (`APT::Color "1"`) for Ubuntu/Debian systems
  - Custom colored PS1 prompt using Tango color scheme (cyan username, gray @, blue hostname, green path)

- Client → Server:

  - Raw text/binary: keystrokes (e.g., `\x03` for Ctrl+C).
  - Resize JSON: `{ "type":"resize", "cols":<int>, "rows":<int> }`.

- Server → Client:
  - Raw binary WebSocket frames that preserve all ANSI/SGR escape sequences without filtering
  - Full support for colors, cursor movement, and other terminal control sequences
  - Applications using ANSI escape codes (like htop, vim, etc.) work properly
  - Support for TrueColor (24-bit RGB) ANSI sequences

## Error format

- UI reads `text()` first; preferable to return human‑readable messages for 4xx/5xx (or JSON `{error:"..."}`).

## Implementation notes

- Prefer Docker API over shelling out to `docker` for proper PTY/IO control.
- WS bridge: parse JSON resize; forward all other bytes to the TTY; stream stdout/stderr back.
- `POST` start/stop may return `204 No Content` (UI also accepts `200`).
- Terminal sessions automatically receive a one-time bootstrap script that:
  - Sets environment variables for proper color support
  - Creates color aliases for common commands
  - Configures APT for colored output (Ubuntu/Debian)
  - Sets a custom PS1 prompt with Tango color scheme
  - This bootstrap is only applied once per terminal session

## Examples

```json
// GET /api/containers → variant A
{
  "containers": [
    {
      "id": "abc123...",
      "name": "web",
      "image": "nginx:1.27",
      "status": "running",
      "labels": { "com.docker.compose.project": "myapp" },
      "ports": [{ "PublicPort": 8080, "PrivatePort": 80, "Type": "tcp" }]
    }
  ]
}
```

```json
// GET /api/containers → variant B
[
  {
    "ID": "abc123...",
    "Names": "/web",
    "Image": "nginx:1.27",
    "State": "running",
    "Labels": { "com.docker.compose.project": "myapp" },
    "Ports": { "80/tcp": [{ "HostIp": "0.0.0.0", "HostPort": "8080" }] }
  }
]
```
