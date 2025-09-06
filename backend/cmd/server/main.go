package main

import (
    "log"
    "net/http"
    "os"
    "time"

    "terminogrid-backend/internal/api"
)

func main() {
    mux := http.NewServeMux()

    // API routes
    s := api.NewServer()
    mux.HandleFunc("/api/containers", s.HandleContainers)
    mux.HandleFunc("/api/containers/", s.HandleContainerActions)
    mux.HandleFunc("/api/health", s.HandleHealth)

    // Serve UI under /ui/ and redirect root to /ui/
    fs := http.FileServer(http.Dir("/ui"))
    mux.Handle("/ui/", http.StripPrefix("/ui/", fs))
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        http.Redirect(w, r, "/ui/", http.StatusFound)
    })

    addr := ":8080"
    if p := os.Getenv("PORT"); p != "" {
        addr = ":" + p
    }

    srv := &http.Server{
        Addr:              addr,
        Handler:           api.WithCORS(mux),
        ReadHeaderTimeout: 5 * time.Second,
    }
    log.Printf("listening on %s", addr)
    log.Fatal(srv.ListenAndServe())
}
