package api

import (
    "context"
    "encoding/json"
    "errors"
    "io"
    "strings"
    "sync"
    "time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
)

type dockerClient struct {
    cli *client.Client
}

func newDockerClient() (*dockerClient, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return nil, err
    }
    // quick ping to verify connectivity
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    _, err = cli.Ping(ctx)
    if err != nil {
        return nil, err
    }
    return &dockerClient{cli: cli}, nil
}

func (d *dockerClient) List(ctx context.Context) ([]Container, error) {
    cs, err := d.cli.ContainerList(ctx, types.ContainerListOptions{All: true})
    if err != nil {
        return nil, err
    }
    out := make([]Container, 0, len(cs))
    for _, c := range cs {
        if isSystemContainer(c) {
            continue
        }
        name := ""
        if len(c.Names) > 0 {
            name = strings.TrimPrefix(c.Names[0], "/")
        }
        labels := map[string]string{}
        for k, v := range c.Labels { labels[k] = v }
        ports := make([]Port, 0, len(c.Ports))
        for _, p := range c.Ports {
            t := p.Type
            if t == "" { t = "tcp" }
            ports = append(ports, Port{PublicPort: int(p.PublicPort), PrivatePort: int(p.PrivatePort), Type: t})
        }
        out = append(out, Container{
            ID:     c.ID,
            Name:   name,
            Image:  c.Image,
            Status: c.State,
            Labels: labels,
            Ports:  ports,
        })
    }
    return out, nil
}

// isSystemContainer filters out TerminoGrid's own containers and helpers.
func isSystemContainer(c types.Container) bool {
    labs := c.Labels
    if strings.EqualFold(strings.TrimSpace(labs["grid.system"]), "true") {
        return true
    }
    // Compose labels/project/service
    proj := strings.ToLower(labs["com.docker.compose.project"])
    svc := strings.ToLower(labs["com.docker.compose.service"])
    if proj != "" && strings.Contains(proj, "terminogrid") { return true }
    if svc != "" && strings.Contains(svc, "terminogrid") { return true }
    // Names
    for _, n := range c.Names {
        n = strings.ToLower(strings.TrimPrefix(n, "/"))
        if n == "terminogrid" || strings.HasPrefix(n, "terminogrid-") || strings.HasPrefix(n, "buildx_buildkit") {
            return true
        }
    }
    // Image
    img := strings.ToLower(c.Image)
    if strings.Contains(img, "terminogrid-backend") || strings.Contains(img, "terminogrid") || strings.Contains(img, "buildkit") {
        return true
    }
    return false
}

func (d *dockerClient) Start(ctx context.Context, id string) error {
    return d.cli.ContainerStart(ctx, id, types.ContainerStartOptions{})
}

func (d *dockerClient) Stop(ctx context.Context, id string) error {
    // Older client signature: timeout pointer
    var timeout *time.Duration
    return d.cli.ContainerStop(ctx, id, timeout)
}

func (d *dockerClient) Exists(ctx context.Context, id string) bool {
    _, err := d.cli.ContainerInspect(ctx, id)
    return err == nil
}

// ExecBridge upgrades docker exec and bridges IO to the provided websocket.
// The optional setupHook is called after attach but before IO pumping begins.
func (d *dockerClient) ExecBridge(ctx context.Context, ws *websocket.Conn, containerID string, setupHook ...func(attach types.HijackedResponse) error) error {
    if !d.Exists(ctx, containerID) {
        return errNotFound
    }
    
    // Prepare an interactive login shell with sensible envs
    shells := [][]string{
        {"/bin/bash", "-li"},
        {"/usr/bin/bash", "-li"},
        {"/bin/sh", "-i"},
        {"/usr/bin/sh", "-i"},
    }
    var resp types.IDResponse
    var err error
    picked := []string{}
    for _, cmd := range shells {
        cfg := types.ExecConfig{
            AttachStdin:  true,
            AttachStdout: true,
            AttachStderr: true,
            Tty:          true,
            Cmd:          strslice.StrSlice(cmd),
            Env:          []string{
                "TERM=xterm-256color", 
                "COLORTERM=truecolor", 
                "CLICOLOR_FORCE=1", 
                "FORCE_COLOR=1",
            },
        }
        resp, err = d.cli.ContainerExecCreate(ctx, containerID, cfg)
        if err == nil && resp.ID != "" {
            picked = cmd
            break
        }
    }
    if err != nil || resp.ID == "" {
        return err
    }
    // Initial size from query if provided via WS preface is not directly available here.
    // The frontend will send resize JSON immediately after open; we’ll react then.

    attach, err := d.cli.ContainerExecAttach(ctx, resp.ID, types.ExecStartCheck{Tty: true})
    if err != nil {
        return err
    }
    defer attach.Close()

    // Inform client which shell was picked
    if len(picked) > 0 {
        _ = ws.WriteMessage(websocket.BinaryMessage, []byte("[shell] " + strings.Join(picked, " ") + "\n"))
    }

    // Execute setup hook if provided
    if len(setupHook) > 0 && setupHook[0] != nil {
        if err := setupHook[0](attach); err != nil {
            return err
        }
    }

    // Pump docker -> ws
    var writeMu sync.Mutex
    dockerToWS := make(chan error, 1)
    go func() {
        buf := make([]byte, 4096)
        for {
            n, readErr := attach.Reader.Read(buf)
            if n > 0 {
                writeMu.Lock()
                werr := ws.WriteMessage(websocket.BinaryMessage, buf[:n])
                writeMu.Unlock()
                if werr != nil {
                    dockerToWS <- werr
                    return
                }
            }
            if readErr != nil {
                if errors.Is(readErr, io.EOF) {
                    dockerToWS <- nil
                } else {
                    dockerToWS <- readErr
                }
                return
            }
        }
    }()

    // Pump ws -> docker (and handle resize)
    wsToDocker := make(chan error, 1)
    go func() {
        for {
            mt, data, rerr := ws.ReadMessage()
            if rerr != nil {
                wsToDocker <- rerr
                return
            }
            if mt == websocket.TextMessage {
                // Try resize JSON
                var msg struct {
                    Type string `json:"type"`
                    Cols int    `json:"cols"`
                    Rows int    `json:"rows"`
                }
                if json.Unmarshal(data, &msg) == nil && strings.ToLower(msg.Type) == "resize" {
                    _ = d.cli.ContainerExecResize(ctx, resp.ID, types.ResizeOptions{Width: uint(msg.Cols), Height: uint(msg.Rows)})
                    continue
                }
            }
            // Forward raw input to container
            if _, werr := attach.Conn.Write(data); werr != nil {
                wsToDocker <- werr
                return
            }
        }
    }()

    // Wait for any side to finish
    select {
    case err = <-dockerToWS:
        return err
    case err = <-wsToDocker:
        // If ws closed, close the docker side
        attach.Close()
        return err
    case <-ctx.Done():
        return ctx.Err()
    }
}
