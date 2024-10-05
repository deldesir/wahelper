package main

import (
    "os"
    "os/signal"
    "syscall"
    "wahelper/whatsapp"
    "wahelper/server"
    "wahelper/handlers"
    "github.com/jessevdk/go-flags"
    "bufio"
    "strings"
    "fmt"
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

    srv := server.NewServer(client)
    srv.Start()

    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-c
        client.Logger.Info("Shutting down...")
        srv.Stop()
        client.Disconnect()
        os.Exit(0)
    }()

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
            go client.HandleCommand(cmdName, args)
        }
    }
}
