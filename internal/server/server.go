package server

import (
    "context"
    "log"
    "net/http"
    "time"

    "resilientrecursion/internal/engine"
)

type Server struct {
    engine *engine.ComputeEngine
    server *http.Server
}

func NewServer(port string, eng *engine.ComputeEngine) *Server {
    s := &Server{engine: eng}
    
    mux := http.NewServeMux()
    mux.HandleFunc("/calculate", s.handleCalculate)
    mux.HandleFunc("/health", s.handleHealth)
    
    s.server = &http.Server{
        Addr:         ":" + port,
        Handler:      mux,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
    }
    
    return s
}

func (s *Server) Start() error {
    log.Printf("Starting server on %s", s.server.Addr)
    return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
    return s.server.Shutdown(ctx)
}