package handlers

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"
    "time"
    "wahelper/whatsapp"
    "wahelper/server"
    "wahelper/utils"
    "context"
    "path/filepath"
    "go.mau.fi/whatsmeow/appstate"
    "strconv"
)

func HandleHTTPRequest(w http.ResponseWriter, r *http.Request, waClient *whatsapp.Client, srv *server.Server) {
    if r.URL.Path != "/" {
        http.Error(w, "404 not found.", http.StatusNotFound)
        waClient.Logger.Errorf("Invalid request path, 404 not found.")
        return
    }

    switch r.Method {
    case "GET":
        if waClient.IsConnected {
            if waClient.Config.Mode == "both" {
                waClient.Logger.Infof("GET request received, server is running in both mode")
                fmt.Fprintf(w, "Server is running in both mode")
            } else if waClient.Config.Mode == "send" {
                waClient.Logger.Infof("GET request received, server is running in send mode")
                fmt.Fprintf(w, "Server is running in send mode")
            }
        } else {
            waClient.Logger.Infof("GET request received, server is waiting for reconnection")
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
                waClient.Logger.Errorf("Error: %s", err)
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
                    srv.Stop()
                    waClient.Logger.Infof("Exit command received, exiting...")
                    waClient.Disconnect()
                    os.Exit(0)
                }()
                return
            } else if cmd == "restart" {
                fmt.Fprintf(w, "restarting")
                go func() {
                    time.Sleep(1 * time.Second)
                    srv.Stop()
                    if waClient.Config.Mode == "both" {
                        waClient.Logger.Infof("Receive/Send Mode Enabled")
                        waClient.Logger.Infof("Will Now Receive/Send Messages")
                        srv.Start()
                    } else if waClient.Config.Mode == "send" {
                        waClient.Logger.Infof("Send Mode Enabled")
                        waClient.Logger.Infof("Can Now Send Messages")
                        srv.Start()
                    }
                }()
                return
            }

            fmt.Fprintf(w, "command received")
            if waClient.Config.Mode == "both" || waClient.Config.Mode == "send" {
                go HandleCommand(waClient, cmd, args[1:])
            }
        }
        return
    default:
        waClient.Logger.Errorf("%s, only GET and POST methods are supported.", r.Method)
        return
    }
}

func HandleCommand(waClient *whatsapp.Client, cmd string, args []string) {
    switch cmd {
    case "send":
        if len(args) < 2 {
            waClient.Logger.Error("Usage: send <jid> <message>")
            return
        }
        recipientJID := args[0]
        message := strings.Join(args[1:], " ")
        err := waClient.SendMessage(recipientJID, message)
        if err != nil {
            waClient.Logger.Errorf("Failed to send message: %v", err)
        }
    case "pair-phone":
        if len(args) < 1 {
            waClient.Logger.Error("Usage: pair-phone <number>")
            return
        }
        if !waClient.WAClient.IsConnected() {
            waClient.Logger.Error("Not connected to WhatsApp")
            return
        }
        if waClient.WAClient.IsLoggedIn() {
            waClient.Logger.Info("Already paired")
            return
        }
        linkingCode, err := waClient.WAClient.PairPhone(args[0], true, whatsmeow.PairClientUnknown, "Firefox (Android)")
        if err != nil {
            waClient.Logger.Errorf("Error pairing phone: %v", err)
            return
        }
        waClient.Logger.Infof(`Linking code: "%s"`, linkingCode)
    case "logout":
        err := waClient.WAClient.Logout()
        if err != nil {
            waClient.Logger.Errorf("Error logging out: %v", err)
        } else {
            waClient.Logger.Infof("Successfully logged out")
        }
    case "appstate":
        if len(args) < 1 {
            waClient.Logger.Error("Usage: appstate <types...>")
            return
        }
        names := []appstate.WAPatchName{appstate.WAPatchName(args[0])}
        if args[0] == "all" {
            names = []appstate.WAPatchName{appstate.WAPatchRegular, appstate.WAPatchRegularHigh, appstate.WAPatchRegularLow, appstate.WAPatchCriticalUnblockLow, appstate.WAPatchCriticalBlock}
        }
        resync := len(args) > 1 && args[1] == "resync"
        for _, name := range names {
            err := waClient.WAClient.FetchAppState(name, resync, false)
            if err != nil {
                waClient.Logger.Errorf("Failed to sync app state: %v", err)
            }
        }
    // TODO: Add other command cases
    default:
        waClient.Logger.Warnf("Unknown command: %s", cmd)
    }
}

