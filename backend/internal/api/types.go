package api

// Container is the normalized shape emitted to the UI.
// It mirrors Docker-ish fields that the frontend accepts.
type Container struct {
    ID     string            `json:"id"`
    Name   string            `json:"name"`
    Image  string            `json:"image"`
    Status string            `json:"status"`
    Labels map[string]string `json:"labels,omitempty"`
    Ports  []Port            `json:"ports,omitempty"`
}

// Port emulates Docker API list entry {PublicPort, PrivatePort, Type}
type Port struct {
    PublicPort  int    `json:"PublicPort,omitempty"`
    PrivatePort int    `json:"PrivatePort,omitempty"`
    Type        string `json:"Type,omitempty"`
}

// TerminalOptions contains configuration options for terminal sessions
// These are optional and backward compatible
type TerminalOptions struct {
    ForceColor   bool   `json:"forceColor,omitempty"`   // Force colors in terminal output (default: true)
    CustomPrompt bool   `json:"customPrompt,omitempty"` // Use custom PS1 prompt (default: true)
    TermEnv      string `json:"termEnv,omitempty"`      // Terminal environment (default: "xterm-256color")
}

