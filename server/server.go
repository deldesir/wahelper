package server

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"
    "time"
    "wahelper/handlers"
    "wahelper/whatsapp"
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
        mux.HandleFunc("/", s.HandleHTTPRequest)
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

func (s *Server) HandleHTTPRequest(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.Error(w, "404 not found.", http.StatusNotFound)
        s.WAClient.Logger.Errorf("Invalid request path, 404 not found.")
        return
    }

    switch r.Method {
    case "GET":
        if s.WAClient.IsConnected {
            if s.WAClient.Config.Mode == "both" {
                s.WAClient.Logger.Infof("GET request received, server is running in both mode")
                fmt.Fprintf(w, "Server is running in both mode")
            } else if s.WAClient.Config.Mode == "send" {
                s.WAClient.Logger.Infof("GET request received, server is running in send mode")
                fmt.Fprintf(w, "Server is running in send mode")
            }
        } else {
            s.WAClient.Logger.Infof("GET request received, server is waiting for reconnection")
            fmt.Fprintf(w, "Bad network, server is waiting for reconnection")
        }
        return
    case "POST":
        dec := json.NewDecoder(r.Body)
        for {
            argsData := struct {
                Args []string `json:"args"`
            }{}

            if err := dec.Decode(&argsData); err == io.EOF {
                break
            } else if err != nil {
                s.WAClient.Logger.Errorf("Error: %s", err)
                return
            }

            args := argsData.Args

            if len(args) < 1 {
                fmt.Fprintf(w, "command received")
                return
            }

            cmd := strings.ToLower(args[0])

            if cmd == "stop" {
                fmt.Fprintf(w, "exiting")
                go func() {
                    time.Sleep(1 * time.Second)
                    s.Stop()
                    s.WAClient.Logger.Infof("Exit command received, exiting...")
                    s.WAClient.Disconnect()
                    os.Exit(0)
                }()
                return
            } else if cmd == "restart" {
                fmt.Fprintf(w, "restarting")
                go func() {
                    time.Sleep(1 * time.Second)
                    s.Stop()
                    if s.WAClient.Config.Mode == "both" {
                        s.WAClient.Logger.Infof("Receive/Send Mode Enabled")
                        s.WAClient.Logger.Infof("Will Now Receive/Send Messages")
                        s.Start()
                    } else if s.WAClient.Config.Mode == "send" {
                        s.WAClient.Logger.Infof("Send Mode Enabled")
                        s.WAClient.Logger.Infof("Can Now Send Messages")
                        s.Start()
                    }
                }()
                return
            }

            fmt.Fprintf(w, "command received")
            if s.WAClient.Config.Mode == "both" || s.WAClient.Config.Mode == "send" {
                go handlers.HandleCommand(s.WAClient, cmd, args[1:])
            }
        }
        return
    default:
        s.WAClient.Logger.Errorf("%s, only GET and POST methods are supported.", r.Method)
        return
    }
}
