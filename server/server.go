package server

import (
    "fmt"
    "net/http"
    "wahelper/whatsapp"
    "wahelper/handlers"
    "time"
    "os"
)

type Server struct {
    WAClient       *whatsapp.Client
    HTTPServer     *http.Server
    ServerRunning  bool
    KillServerChan chan struct{}
}

func NewServer(waClient *whatsapp.Client) *Server {
    return &Server{
        WAClient:       waClient,
        ServerRunning:  false,
        KillServerChan: make(chan struct{}),
    }
}

func (s *Server) Start() {
    if !s.ServerRunning {
        mux := http.NewServeMux()
        mux.HandleFunc("/", s.mdtestHandler)
        s.HTTPServer = &http.Server{
            Addr:    "localhost:" + fmt.Sprintf("%d", s.WAClient.Config.HTTPPort),
            Handler: mux,
        }
        s.ServerRunning = true
        s.WAClient.Logger.Infof("HTTP server started")
        go func() {
            err := s.HTTPServer.ListenAndServe()
            if err != nil && err != http.ErrServerClosed {
                s.WAClient.Logger.Errorf("HTTP server error: %v", err)
            }
            s.ServerRunning = false
        }()
    }
}

func (s *Server) Stop() {
    if s.ServerRunning {
        s.HTTPServer.Close()
        s.ServerRunning = false
        s.WAClient.Logger.Infof("HTTP server stopped")
    }
}

func (s *Server) mdtestHandler(w http.ResponseWriter, r *http.Request) {
    handlers.HandleHTTPRequest(w, r, s.WAClient, s)
}
