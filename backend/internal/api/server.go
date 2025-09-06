package api

import (
    "encoding/json"
    "errors"
    "log"
    "net/http"
    "regexp"
    "strconv"
    "strings"
    "sync"
    "time"

	"github.com/docker/docker/api/types"
	"github.com/gorilla/websocket"
)

// WithCORS adds simple permissive CORS for dev/testing.
func WithCORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}

// sessionState tracks active terminal sessions
type sessionState struct {
    bootstrapSent bool // Whether terminal bootstrap has been sent already
}

type Server struct {
    store         *memStore
    dcli          *dockerClient // nil => mem/demo mode
    activeSessions map[string]*sessionState // Track active terminal sessions
    sessionMutex   sync.Mutex // Protect concurrent access to the sessions map
}

func NewServer() *Server {
    s := &Server{
        store: newMemStore(),
        activeSessions: make(map[string]*sessionState),
    }
    if dc, err := newDockerClient(); err == nil {
        s.dcli = dc
        log.Println("docker client initialized; using real backend")
    } else {
        // Do not crash; keep nil to signal 503 on API calls
        log.Printf("docker client unavailable: %v", err)
    }
    return s
}

// HandleContainers serves GET /api/containers
func (s *Server) HandleContainers(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        s.listContainers(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

// HandleContainerActions multiplexes
//  POST /api/containers/{id}/start
//  POST /api/containers/{id}/stop
//  WS   /api/containers/{id}/exec
func (s *Server) HandleContainerActions(w http.ResponseWriter, r *http.Request) {
    path := strings.TrimPrefix(r.URL.Path, "/api/containers/")
    // exec is a websocket endpoint: /api/containers/{id}/exec
    if strings.HasSuffix(path, "/exec") {
        id := strings.TrimSuffix(path, "/exec")
        id = strings.TrimSuffix(id, "/")
        s.execWS(w, r, id)
        return
    }
    // start/stop are POST endpoints
    re := regexp.MustCompile(`^([^/]+)/([^/]+)$`)
    m := re.FindStringSubmatch(path)
    if len(m) != 3 {
        http.NotFound(w, r)
        return
    }
    id, action := m[1], m[2]
    switch r.Method {
    case http.MethodPost:
        switch action {
        case "start":
            s.startContainer(w, r, id)
        case "stop":
            s.stopContainer(w, r, id)
        default:
            http.NotFound(w, r)
        }
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

// GET /api/containers
func (s *Server) listContainers(w http.ResponseWriter, r *http.Request) {
    if s.dcli == nil {
        http.Error(w, "Docker daemon unavailable (mount /var/run/docker.sock)", http.StatusServiceUnavailable)
        return
    }
    out, err := s.dcli.List(r.Context())
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    // UI akzeptiert sowohl Array als auch {containers:[]} — wir liefern letzteres
    _ = json.NewEncoder(w).Encode(map[string]any{"containers": out})
}

func (s *Server) startContainer(w http.ResponseWriter, r *http.Request, id string) {
    if s.dcli == nil { http.Error(w, "Docker daemon unavailable", http.StatusServiceUnavailable); return }
    if err := s.dcli.Start(r.Context(), id); err != nil { statusErr(w, err); return }
    w.WriteHeader(http.StatusNoContent)
}

func (s *Server) stopContainer(w http.ResponseWriter, r *http.Request, id string) {
    if s.dcli == nil { http.Error(w, "Docker daemon unavailable", http.StatusServiceUnavailable); return }
    if err := s.dcli.Stop(r.Context(), id); err != nil { statusErr(w, err); return }
    w.WriteHeader(http.StatusNoContent)
}

// WebSocket demo exec bridge.
var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// Terminal bootstrap script to be sent once per terminal session
const termBootstrapScript = `export TERM=xterm-256color COLORTERM=truecolor CLICOLOR_FORCE=1 FORCE_COLOR=1; \
alias ls='ls --color=auto'; alias grep='grep --color=auto' 2>/dev/null || true; \
export LESS='-R'; \
if [ -d /etc/apt/apt.conf.d ]; then printf 'APT::Color "1";\n' > /etc/apt/apt.conf.d/99tg-color; fi; \
PS1='\[\e[38;2;52;226;226m\]\u\[\e[0m\]\[\e[38;2;85;87;83m\]@\[\e[0m\]\[\e[38;2;114;159;207m\]\h\[\e[0m\] \[\e[38;2;138;226;52m\]\w\[\e[0m\] \$ '
`

func (s *Server) execWS(w http.ResponseWriter, r *http.Request, id string) {
    if s.dcli == nil {
        http.Error(w, "Docker daemon unavailable", http.StatusServiceUnavailable)
        return
    }
    // Upgrade first; error responses after WS upgrade must be sent over WS
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("ws upgrade: %v", err)
        return
    }
    defer conn.Close()
    
    // Track session state
    sessionID := id + "-" + time.Now().Format("20060102-150405.000")
    s.sessionMutex.Lock()
    session := &sessionState{bootstrapSent: false}
    s.activeSessions[sessionID] = session
    s.sessionMutex.Unlock()
    
    // Clean up session when done
    defer func() {
        s.sessionMutex.Lock()
        delete(s.activeSessions, sessionID)
        s.sessionMutex.Unlock()
    }()

    // Setup terminal hook to inject bootstrap after connection
    termSetupHook := func(attach types.HijackedResponse) error {
        // Send bootstrap script only once per session
        s.sessionMutex.Lock()
        bootstrapSent := session.bootstrapSent
        if !bootstrapSent {
            session.bootstrapSent = true
        }
        s.sessionMutex.Unlock()
        
        if !bootstrapSent {
            // Send bootstrap commands to setup color and prompt
            // Small delay to ensure shell is ready
            time.Sleep(100 * time.Millisecond)
            _, err := attach.Conn.Write([]byte(termBootstrapScript + "\n"))
            if err != nil {
                return err
            }
        }
        return nil
    }

    if s.dcli != nil {
        // Pass the terminal setup hook to ExecBridge
        if err := s.dcli.ExecBridge(r.Context(), conn, id, termSetupHook); err != nil {
            _ = conn.WriteMessage(websocket.BinaryMessage, []byte("[exec error] "+err.Error()+"\n"))
        }
        return
    }

    // Fallback: Demo echo
    _ = conn.WriteMessage(websocket.BinaryMessage, []byte("Demo-Session verbunden. Eingaben werden ge-echoed.\n"))
    for {
        mt, data, err := conn.ReadMessage()
        if err != nil { return }
        if mt == websocket.TextMessage {
            var msg struct{ Type string `json:"type"`; Cols, Rows int }
            if json.Unmarshal(data, &msg) == nil && strings.ToLower(msg.Type) == "resize" {
                note := []byte(time.Now().Format("15:04:05 ")+" resize "+toPair(msg.Cols, msg.Rows)+"\n")
                _ = conn.WriteMessage(websocket.BinaryMessage, note)
                continue
            }
        }
        _ = conn.WriteMessage(websocket.BinaryMessage, data)
    }
}

// Health endpoint: 200 when server is up; 503 if docker unavailable.
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
    if s.dcli == nil {
        http.Error(w, "docker unavailable", http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
}

func toPair(c, r int) string { return strconv.Itoa(c) + "x" + strconv.Itoa(r) }

func statusErr(w http.ResponseWriter, err error) {
    var code = http.StatusInternalServerError
    if errors.Is(err, errNotFound) {
        code = http.StatusNotFound
    }
    http.Error(w, err.Error(), code)
}
