package gophermc_test

import (
	"context"
	"fmt"
	"github.com/SyNdicateFoundation/GopherMc"
	"github.com/SyNdicateFoundation/GopherMc/protocol"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	serverHost = "127.0.0.1"
	serverPort = 36000
)

func TestEventListener(t *testing.T) {
	if testing.Short() {
		t.Skipf("Skipping test, requires long-running server")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := gophermc.NewClient(
		gophermc.WithUsername("GopherMC"),
		gophermc.WithAddr(serverHost+":"+strconv.Itoa(serverPort)),
	)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	defer client.Close()

	events, err := client.JoinAndListen(ctx, 10)
	if err != nil {
		t.Fatalf("JoinAndListen failed: %v", err)
	}
	t.Log("Login successful, listening for events...")

	for {
		select {
		case event := <-events:
			if event == nil {
				t.Log("Event channel closed, test finished.")
				return
			}
			t.Logf("Received event: %T %+v", event, event)
			switch event.(type) {
			case gophermc.ReadyEvent:
				t.Log("Client is ready!")
			case gophermc.KeepAliveEvent:
				t.Log("Successfully received a keep-alive event. Test passed.")
				return
			case gophermc.DisconnectEvent:
				t.Error("Got disconnected unexpectedly.")
				return
			}
		case <-ctx.Done():
			t.Fatal("Test timed out waiting for a KeepAlive event.")
		}
	}
}

func TestJoinAndChatAllVersions(t *testing.T) {
	const timeout = 20 * time.Second

	for v := protocol.First; v <= protocol.Latest; v++ {
		version := v
		t.Run(version.String(), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			username := "GopherMC_" + strings.ReplaceAll(version.String(), ".", "_")
			chatMessage := fmt.Sprintf("hi from version %s", version)

			err := SendChat(ctx, version, serverHost, serverPort, username, chatMessage)
			if err != nil {
				if strings.Contains(err.Error(), "unsupported protocol version") {
					t.Skipf("Server does not support version %s", version)
				}
				t.Fatalf("SendChat failed for %s: %v", version, err)
			}

			t.Logf("OK! Sent chat as %s on version %s.", username, version)
		})
	}
}

func SendChat(ctx context.Context, version protocol.Version, host string, port uint16, username, message string) error {
	client, err := gophermc.NewClient(
		gophermc.WithVersion(version),
		gophermc.WithUsername(username),
		gophermc.WithAddr(host+":"+strconv.Itoa(int(port))),
	)
	if err != nil {
		return fmt.Errorf("failed to create client for chat: %w", err)
	}

	defer client.Destroy()

	if err := client.Join(ctx); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	time.Sleep(time.Second * 2)

	if err := client.Chat(message); err != nil {
		return fmt.Errorf("sending chat failed: %w", err)
	}

	time.Sleep(time.Second * 2)

	return nil
}
