package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "wahelper/handlers"
    "wahelper/server"
    "wahelper/whatsapp"
)

func main() {
    httpPort := flag.Int("port", 7774, "HTTP server port")
    mode := flag.String("mode", "none", "Select mode: none, both, send")
    saveMedia := flag.Bool("save-media", false, "Save media")
    autoDelete := flag.Bool("auto-delete-media", false, "Delete downloaded media after 30s")
    debugLogs := flag.Bool("debug", false, "Enable debug logs")
    dbDialect := flag.String("db-dialect", "sqlite3", "Database dialect (sqlite3 or postgres)")
    dbAddress := flag.String("db-address", "file:wahelper.db?_foreign_keys=on", "Database address")
    requestFullSync := flag.Bool("request-full-sync", false, "Request full (1 year) history sync when logging in")
    flag.Parse()

    logLevel := "INFO"
    if *debugLogs {
        logLevel = "DEBUG"
    }
    logger := log.New(os.Stdout, "", log.LstdFlags)

    waClient, err := whatsapp.NewClient(*dbDialect, *dbAddress, logLevel, *requestFullSync, logger)
    if err != nil {
        logger.Fatalf("Failed to initialize WhatsApp client: %v", err)
    }

    waClient.SetEventHandler(handlers.HandleEvent)

    err = waClient.Connect()
    if err != nil {
        logger.Fatalf("Failed to connect to WhatsApp: %v", err)
    }

    args := flag.Args()
    if len(args) > 0 {
        cmd := args[0]
        cmdArgs := args[1:]
        handlers.HandleCommand(waClient, cmd, cmdArgs)
        // If the command is not "pair-phone", exit after handling
        if cmd != "pair-phone" {
            return
        }
    } else {
        // Check if the client is logged in
        if !waClient.IsLoggedIn() {
            fmt.Fprintln(os.Stderr, "If not paired, try running:\n\n./wahelper pair-phone <number>\n\n<number> is \"Country Code\" + \"Phone Number\"\n(e.g., if Country Code = 91, then use 919876543210)")
            os.Exit(1)
        }
    }

    // Start HTTP server if in "both" or "send" mode
    if *mode == "both" || *mode == "send" {
        go func() {
            err := server.StartServer(*httpPort, handlers.GetHTTPHandler(waClient, *mode, *saveMedia, *autoDelete))
            if err != nil {
                logger.Fatalf("Failed to start HTTP server: %v", err)
            }
        }()
        logger.Printf("%s mode enabled", *mode)
    }

    // Wait for interrupt signal to gracefully shut down
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    <-c
    logger.Println("Shutting down...")
    waClient.Disconnect()
}
