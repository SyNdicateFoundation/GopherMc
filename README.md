<h1 align="center">
<img src="https://i.imgur.com/jxf2qJq.png" align="center" width="256">
<br>
</h1>

#  GopherMc

GopherMc is a powerful and flexible Go library for creating Minecraft clients (bots). It provides a clean, high-level
API for interacting with Minecraft servers, supporting a wide range of protocol versions from **1.7** to the latest
releases.

[![Go Report Card](https://goreportcard.com/badge/github.com/SyNdicateFoundation/GopherMc)](https://goreportcard.com/report/github.com/SyNdicateFoundation/GopherMc)

## ✨ Features

- **Multi-Version Support**: Connect to servers running anything from Minecraft 1.7 to the latest versions.
- **Event-Driven**: A robust event handling system for receiving server-side events like chat messages, keep-alive, and
  disconnects.
- **Server List Ping**: Get the status (MOTD, player count) and latency of a server.
- **Offline Mode Authentication**: Simple login for offline-mode servers.
- **Chat**: Easily send and receive chat messages.
- **Player Actions**: Send movement, rotation, and action packets to interact with the world.
- **Modern Protocol Handling**: Full support for the `Configuration` state introduced in Minecraft 1.20.2.
- **Clean and Idiomatic Go**: Designed to be easy to use and integrate into your Go projects.

## 📦 Installation

To add GopherMc to your project, simply use `go get`:

```sh
go get github.com/SyNdicateFoundation/GopherMc
```

## 🚀 Usage Examples

Here are some examples of how to use the GopherMc library.

### 1. Get Server Status

Perform a server list ping to get the JSON status response and latency.

```go
package main

import (
	"context"
	"fmt"
	"github.com/SyNdicateFoundation/GopherMc"
	"log"
	"time"
)

func main() {
	host := "127.0.0.1"
	port := uint16(25565)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	client, err := GopherMc.NewClient(
		GopherMc.WithAddr(fmt.Sprintf("%s:%d", host, port)),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	statusJSON, latency, err := client.GetStatus(ctx)
	if err != nil {
		log.Fatalf("Failed to get server status: %v", err)
	}
	
	fmt.Printf("Server Status:\n%s\n", statusJSON)
	fmt.Printf("Latency: %v\n", latency)
}
```

### 2. Send a Chat Message

Connect, log in, send a chat message, and then disconnect.

```go
package main

import (
	"context"
	"fmt"
	"github.com/SyNdicateFoundation/GopherMc"
	"github.com/SyNdicateFoundation/GopherMc/protocol"
	"log"
	"time"
)

func main() {
	host := "127.0.0.1"
	port := uint16(25565)
	username := "GopherBot"
	message := "Hello from GopherMc!"
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	client, err := GopherMc.NewClient(
		GopherMc.WithAddr(fmt.Sprintf("%s:%d", host, port)),
		GopherMc.WithUsername(username),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	
	if err := client.Join(ctx); err != nil {
		log.Fatalf("Failed to join server: %v", err)
	}
	defer client.Destroy()
	
	// Wait a moment for the server to process the login
	time.Sleep(time.Second * 2)
	
	if err := client.Chat(message); err != nil {
		log.Fatalf("Sending chat failed: %v", err)
	}
	
	log.Println("Successfully sent chat message!")
}
```

### 3. Advanced Client with Event Handling

For more complex bots, create a persistent client to listen for server events, receive chat, and send player actions.

```go
package main

import (
	"context"
	"fmt"
	"github.com/SyNdicateFoundation/GopherMc"
	"log"
)

func main() {
	host := "127.0.0.1"
	port := uint16(25565)
	username := "GopherMC"
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	client, err := GopherMc.NewClient(
		GopherMc.WithAddr(fmt.Sprintf("%s:%d", host, port)),
		GopherMc.WithUsername(username),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	
	// Login and start listening for events
	events, err := client.JoinAndListen(ctx, 100)
	if err != nil {
		log.Fatalf("LoginAndListen failed: %v", err)
	}
	defer client.Destroy()
	
	log.Println("Login successful, listening for events...")
	
	// Event loop
	for {
		select {
		case event := <-events:
			if event == nil {
				log.Println("Event channel closed. Exiting.")
				return
			}
			
			// Handle different event types
			switch e := event.(type) {
			case GopherMc.ReadyEvent:
				log.Printf("Client is ready! Logged in as %s\n", e.Username)
			
			case GopherMc.ChatMessageEvent:
				fmt.Printf("[Chat] <%s>: %s\n", e.Sender, e.Message)
			case GopherMc.KeepAliveEvent:
				log.Printf("Received KeepAlive (ID: %d). Client is responding automatically.\n", e.ID)
			case GopherMc.DisconnectEvent:
				log.Printf("Disconnected by server: %s\n", e.Reason)
				return
			default:
				log.Printf("Received unhandled event: %T\n", e)
			}
		case <-ctx.Done():
			log.Println("Program context finished.")
			return
		}
	}
}
```

---

### 🔬 Running Tests

The project includes a suite of tests. The integration tests that connect to a real Minecraft server are skipped by
default. To run them, you must first:

1. Have a local offline-mode Minecraft server running on `127.0.0.1:36000`.
2. Uncomment the `t.Skip(...)` line in the test files (`client_test.go`).
3. Run the tests using the standard Go toolchain:

```sh
go test ./... -v
```

### 🤝 Contributing

Contributions are welcome! Feel free to open an issue to discuss a new feature or bug, or submit a pull request with
your improvements.

### 📜 License

This project is licensed under the **MIT License**.

### ©️ Credits:

We are using [minecraft-data](https://github.com/PrismarineJS/minecraft-data) for generate packet ids of each version