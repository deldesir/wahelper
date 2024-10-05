package main

import (
    "bufio"
    "fmt"
    "os"
    "os/signal"
    "strings"
    "syscall"
    "time"
    "wahelper/server"
    "wahelper/whatsapp"

    "github.com/jessevdk/go-flags"
)

func main() {
    var config whatsapp.Config
    parser := flags.NewParser(&config, flags.Default)
    _, err := parser.Parse()
    if err != nil {
        os.Exit(1)
    }

    client, err := whatsapp.NewClient(&config)
    if err != nil {
        fmt.Printf("Failed to initialize WhatsApp client: %v\n", err)
        os.Exit(1)
    }

    err = client.Connect()
    if err != nil {
        fmt.Printf("Failed to connect to WhatsApp: %v\n", err)
        os.Exit(1)
    }

    // Start the server if mode is "both" or "send"
    if config.Mode == "both" || config.Mode == "send" {
        go server.StartServer(client)
    }

    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-c
        client.Logger.Info("Shutting down...")
        if config.Mode == "both" || config.Mode == "send" {
            server.StopServer()
        }
        client.Disconnect()
        os.Exit(0)
    }()

    // Handle command-line arguments for immediate commands
    args := os.Args[1:]
    if len(args) > 0 {
        cmd := strings.ToLower(args[0])
        if cmd != "pair-phone" {
            go func() {
                for {
                    if client.WAClient.IsConnected() {
                        break
                    }
                    time.Sleep(1 * time.Second)
                }
                time.Sleep(2 * time.Second)
                if !client.WAClient.IsLoggedIn() {
                    fmt.Fprintln(os.Stderr, "If not paired, try running:\n\n./wahelper pair-phone <number>\n\n<number> is \"Country Code\" + \"Phone Number\"\n(e.g., if Country Code = 91, then use 919876543210)")
                    os.Exit(1)
                }
            }()
        }
        handlers.HandleCommand(client, cmd, args[1:])
        if cmd != "pair-phone" {
            return
        }
    } else {
        go func() {
            for {
                if client.WAClient.IsConnected() {
                    break
                }
                time.Sleep(1 * time.Second)
            }
            time.Sleep(2 * time.Second)
            if !client.WAClient.IsLoggedIn() {
                fmt.Fprintln(os.Stderr, "If not paired, try running:\n\n./wahelper pair-phone <number>\n\n<number> is \"Country Code\" + \"Phone Number\"\n(e.g., if Country Code = 91, then use 919876543210)")
                os.Exit(1)
            }
        }()
    }

    input := make(chan string)
    go func() {
        defer close(input)
        scan := bufio.NewScanner(os.Stdin)
        for scan.Scan() {
            line := strings.TrimSpace(scan.Text())
            if len(line) > 0 {
                input <- line
            }
        }
    }()

    for {
        select {
        case cmd := <-input:
            if len(cmd) == 0 {
                client.Logger.Infof("Stdin closed, exiting")
                client.Disconnect()
                return
            }
            args := strings.Fields(cmd)
            cmdName := strings.ToLower(args[0])
            args = args[1:]
            go handlers.HandleCommand(client, cmdName, args)
        }
    }
}
