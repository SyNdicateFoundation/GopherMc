package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/SyNdicateFoundation/GopherMc"
	"github.com/SyNdicateFoundation/GopherMc/protocol"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	serverHost := flag.String("host", "127.0.0.1:36000", "Minecraft server host")
	username := flag.String("username", "GopherBot", "Username to use for login")
	versionStr := flag.String("version", "latest", "Minecraft version string (e.g., 1.18.2, latest)")
	flag.Parse()

	mcVersion, ok := protocol.VersionFromString(*versionStr)
	if !ok && *versionStr != "latest" {
		log.Fatalf("Unknown Minecraft version: %s", *versionStr)
	}

	if *versionStr == "latest" {
		mcVersion = protocol.Latest
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := gophermc.NewClient(
		gophermc.WithAddr(*serverHost),
		gophermc.WithVersion(mcVersion),
		gophermc.WithUsername(*username),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
	defer connectCancel()
	if err := client.Connect(connectCtx); err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}

	defer client.Destroy()

	events, err := client.JoinAndListen(ctx, 100)
	if err != nil {
		log.Fatalf("Failed to login: %v", err)
	}

	log.Printf("Logged in as %s on version %s. You can now send chat messages.", *username, mcVersion.String())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			text := scanner.Text()
			if text == "" {
				continue
			}

			if err := client.Chat(text); err != nil {
				log.Printf("Failed to send chat message: %v", err)
			}
		}
	}()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				log.Println("Event channel closed. Exiting.")
				return
			}

			switch e := event.(type) {
			case gophermc.ReadyEvent:
				log.Printf("Client is ready!")
			case gophermc.DisconnectEvent:
				log.Printf("Disconnected: %s", e.Reason)
				return
			case gophermc.ChatMessageEvent:
				fmt.Printf("[Chat] %s\n", e.Message)
			}
		case <-ctx.Done():
			log.Println("Context done. Exiting.")
			return
		}
	}
}
