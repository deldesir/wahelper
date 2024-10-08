package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"wahelper/whatsapp"
	"wahelper/utils"

	"github.com/jessevdk/go-flags"
)

func main() {
	// Parse command-line flags into the Config struct
	var config whatsapp.Config
	parser := flags.NewParser(&config, flags.Default)
	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	// Initialize the WhatsApp client
	client, err := whatsapp.NewClient(&config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize WhatsApp client: %v\n", err)
		os.Exit(1)
	}

	// Connect the client
	err = client.Connect()
	if err != nil {
		client.Logger.Errorf("Failed to connect to WhatsApp: %v", err)
		os.Exit(1)
	}

	// Start the server if mode is "both" or "send"
	if config.Mode == "both" || config.Mode == "send" {
		go client.StartServer()
	}

	// Handle OS signals for graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		client.Logger.Info("Shutting down...")
		if config.Mode == "both" || config.Mode == "send" {
			client.StopServer()
		}
		client.Disconnect()
		os.Exit(0)
	}()

	// Check for immediate commands provided as command-line arguments
	args := os.Args[1:]
	if len(args) > 0 {
		cmd := strings.ToLower(args[0])

		// Wait until the client is connected before executing commands
		for !client.WAClient.IsConnected() {
			time.Sleep(1 * time.Second)
		}

		// If not logged in, prompt to pair (unless the command is "pair-phone")
		if !client.WAClient.IsLoggedIn() && cmd != "pair-phone" {
			fmt.Fprintln(os.Stderr, "Not logged in. Please pair your device using:\n\n./wahelper pair-phone <number>\n\n<number> is \"Country Code\" + \"Phone Number\"\n(e.g., if Country Code = 91, then use 919876543210)")
			os.Exit(1)
		}

		// Handle the immediate command
		client.HandleCommand(cmd, args[1:])

		// Exit after handling the immediate command (unless it's "pair-phone")
		if cmd != "pair-phone" {
			return
		}
	}

	// Read commands from stdin for interactive mode
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

	// Process commands from stdin
	for {
		select {
		case cmdLine, ok := <-input:
			if !ok || len(cmdLine) == 0 {
				client.Logger.Infof("Stdin closed, exiting")
				client.Disconnect()
				return
			}
			args := strings.Fields(cmdLine)
			if len(args) == 0 {
				continue
			}
			cmdName := strings.ToLower(args[0])
			args = args[1:]

			// Wait until the client is connected before executing commands
			for !client.WAClient.IsConnected() {
				time.Sleep(1 * time.Second)
			}

			// If not logged in, prompt to pair (unless the command is "pair-phone")
			if !client.WAClient.IsLoggedIn() && cmdName != "pair-phone" {
				fmt.Fprintln(os.Stderr, "Not logged in. Please pair your device using:\n\n./wahelper pair-phone <number>\n\n<number> is \"Country Code\" + \"Phone Number\"\n(e.g., if Country Code = 91, then use 919876543210)")
				continue
			}

			// Handle the command
			client.HandleCommand(cmdName, args)
		}
	}
}
