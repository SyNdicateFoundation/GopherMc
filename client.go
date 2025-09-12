package gophermc

import (
	"bytes"
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"github.com/SyNdicateFoundation/GopherMc/protocol"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	*protocol.Conn
	version protocol.Version

	addr *net.TCPAddr

	privateKey *rsa.PrivateKey

	eventChan  chan Event
	readerCtx  context.Context
	cancelRead context.CancelFunc
	readerWg   sync.WaitGroup

	serverHostname string

	brand string

	username string
	uniqueId uuid.UUID

	settings       protocol.ClientSettings
	playerPosition *protocol.PlayerPosition
}

var publicDialler = &net.Dialer{}

func NewClient(opts ...ClientOption) (*Client, error) {
	readerCtx, cancelRead := context.WithCancel(context.Background())

	c := &Client{
		version:        protocol.Latest,
		readerCtx:      readerCtx,
		cancelRead:     cancelRead,
		brand:          "vanilla",
		username:       "GopherMC",
		playerPosition: new(protocol.PlayerPosition),
		settings: protocol.ClientSettings{
			Locale:     "en_US",
			View:       10,
			ChatMode:   0,
			ChatColors: true,
			SkinParts:  0x7F,
			MainHand:   1,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func (c *Client) Connect(ctx context.Context) error {
	if c.Conn != nil {
		_ = c.Close()
	}

	netConn, err := publicDialler.DialContext(ctx, "tcp", c.addr.String())
	if err != nil {
		return fmt.Errorf("failed to dial server: %w", err)
	}

	c.Conn = protocol.NewConn(netConn, c.version)

	return nil
}

func (c *Client) Close() error {
	if c.Conn == nil {
		return nil
	}

	err := c.Conn.Close()

	c.Conn = nil

	return err
}

func (c *Client) Destroy() error {
	c.cancelRead()
	c.readerWg.Wait()

	if c.eventChan != nil {
		close(c.eventChan)
		c.eventChan = nil
	}

	return c.Close()
}

func (c *Client) Events() <-chan Event {
	return c.eventChan
}

func (c *Client) GetStatus(ctx context.Context) (string, time.Duration, error) {
	if err := c.Connect(ctx); err != nil {
		return "", 0, err
	}

	if err := c.SendHandshake(protocol.StateStatus); err != nil {
		return "", 0, fmt.Errorf("sending handshake failed: %w", err)
	}

	c.SetState(protocol.StateStatus)

	if err := c.WritePacket(&protocol.ServerboundStatusRequest{}); err != nil {
		return "", 0, fmt.Errorf("sending status request failed: %w", err)
	}

	p, err := c.ReadPacket()
	if err != nil {
		return "", 0, fmt.Errorf("reading status response failed: %w", err)
	}

	statusResp, ok := p.(*protocol.ClientboundStatusResponse)
	if !ok {
		return "", 0, fmt.Errorf("expected status response, got %T", p)
	}

	latency, err := c.Ping()
	if err != nil {
		return "", 0, err
	}

	return statusResp.JSONResponse, latency, nil
}

func (c *Client) Ping() (time.Duration, error) {
	if c.Conn == nil {
		return 0, fmt.Errorf("client not connected")
	}

	startTime := time.Now()
	pingPacket := &protocol.ServerboundPing{Payload: startTime.UnixMilli()}

	if err := c.WritePacket(pingPacket); err != nil {
		return 0, fmt.Errorf("sending ping failed: %w", err)
	}

	p, err := c.ReadPacket()
	if err != nil {
		return 0, fmt.Errorf("reading pong failed: %w", err)
	}

	if _, ok := p.(*protocol.ClientboundPong); !ok {
		return 0, fmt.Errorf("expected pong response, got %T", p)
	}

	return time.Since(startTime), nil
}

func (c *Client) Join(ctx context.Context) error {
	if err := c.Connect(ctx); err != nil {
		return err
	}

	if err := c.SendHandshake(protocol.StateLogin); err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}

	c.SetState(protocol.StateLogin)

	if err := c.SendLogin(c.username, c.uniqueId); err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}

	for {
		packet, err := c.ReadPacket()
		if err != nil {
			return fmt.Errorf("error during login sequence: %w", err)
		}

		if packet == nil {
			continue
		}

		switch p := packet.(type) {
		case *protocol.ClientboundSetCompression:
			c.SetCompression(int(p.Threshold))

		case *protocol.ClientboundDisconnect:
			return fmt.Errorf("disconnected by server: %s, %s", c.State(), p.Reason)

		case *protocol.ClientboundLoginSuccess:
			if c.version >= protocol.V1_20_2 {
				ack := &protocol.ServerboundLoginAcknowledged{}
				if err := c.WritePacket(ack); err != nil {
					return fmt.Errorf("failed to send login acknowledged: %w", err)
				}
				c.SetState(protocol.StateConfiguration)
				return c.handleConfiguration()
			}

			c.SetState(protocol.StatePlay)
			return c.SendClientSettings(c.settings)

		case *protocol.ClientboundJoinGame:
			return nil

		case *protocol.ClientboundKeepAlive:
			ka := &protocol.ServerboundKeepAlive{ID: p.ID}
			if err := c.WritePacket(ka); err != nil {
				return fmt.Errorf("failed to respond to keep alive during login: %w", err)
			}
		default:
			log.Printf("[INFO] Ignoring unexpected packet during login sequence: %T", p)
		}
	}
}
func (c *Client) JoinAndListen(ctx context.Context, eventCount int) (<-chan Event, error) {
	if err := c.Join(ctx); err != nil {
		return nil, err
	}

	if c.eventChan != nil {
		close(c.eventChan)
		c.eventChan = make(chan Event, eventCount)
	} else {
		c.eventChan = make(chan Event, eventCount)
	}

	c.readerWg.Wait()
	c.readerWg.Add(1)

	go c.readLoop()

	c.eventChan <- ReadyEvent{Username: c.username}

	return c.Events(), nil
}

func (c *Client) Chat(message string) error {
	if c.Conn == nil {
		return fmt.Errorf("client not connected")
	}

	if c.State() != protocol.StatePlay {
		return errors.New("client is not in the play state")
	}

	packet := &protocol.ServerboundChatMessage{
		Message:    message,
		PrivateKey: c.privateKey,
		UUID:       c.uniqueId,
	}

	return c.WritePacket(packet)
}

func (c *Client) SetPosition(x, y, z float64, yaw, headYaw, pitch float32, onGround bool) error {
	if c.Conn == nil {
		return fmt.Errorf("client not connected")
	}

	if c.State() != protocol.StatePlay {
		return errors.New("client is not in the play state")
	}

	c.playerPosition.Update(x, y, z, yaw, headYaw, pitch, onGround)

	packet := &protocol.ServerboundPlayerPosition{
		X:        c.playerPosition.X,
		Y:        c.playerPosition.Y,
		Z:        c.playerPosition.Z,
		OnGround: c.playerPosition.OnGround,
	}

	return c.WritePacket(packet)
}

func (c *Client) readLoop() {
	defer c.readerWg.Done()

	for {
		select {
		case <-c.readerCtx.Done():
			return
		default:
			packet, err := c.ReadPacket()
			if err != nil {
				if errors.Is(err, protocol.ErrUnknownPacket) {
					continue
				}

				if !errors.Is(err, io.EOF) {
					log.Printf("Error reading packet: %v", err)
				}

				if c.eventChan != nil {
					c.eventChan <- DisconnectEvent{Reason: err.Error()}
				}

				return
			}
			c.handlePacket(packet)
		}
	}
}

func (c *Client) handlePacket(packet protocol.Packet) {
	if packet == nil {
		return
	}

	switch p := packet.(type) {
	case *protocol.ClientboundKeepAlive:
		ka := &protocol.ServerboundKeepAlive{ID: p.ID}
		if err := c.WritePacket(ka); err != nil {
			log.Printf("Failed to send keep-alive response: %v", err)
		}

		if c.eventChan != nil {
			c.eventChan <- KeepAliveEvent{ID: p.ID}
		}

	case *protocol.ClientboundChatMessage:
		if c.eventChan != nil {
			c.eventChan <- ChatMessageEvent{
				Component: p.Component,
				Message:   p.Component.String(),
				Sender:    p.Sender,
				Time:      time.Now(),
			}
		}

	case *protocol.ClientboundDisconnect:
		if c.eventChan != nil {
			c.eventChan <- DisconnectEvent{Reason: p.Reason}
		}

		c.cancelRead()
	}
}

func (c *Client) handleConfiguration() error {
	if err := c.SendClientSettings(c.settings); err != nil {
		return fmt.Errorf("failed to send client settings in config: %w", err)
	}

	for {
		packet, err := c.ReadPacket()
		if err != nil {
			if errors.Is(err, protocol.ErrUnknownPacket) {
				continue
			}
			return fmt.Errorf("error during configuration: %w", err)
		}
		if packet == nil {
			continue
		}

		switch p := packet.(type) {
		case *protocol.ClientboundConfigKeepAlive:
			keepAlive := &protocol.ServerboundConfigKeepAlive{ID: p.ID}
			if err := c.WritePacket(keepAlive); err != nil {
				return fmt.Errorf("failed to send config keep alive: %w", err)
			}

		case *protocol.ClientboundCustomPayload:
			if p.Channel == "minecraft:brand" {
				var buf bytes.Buffer
				if err := protocol.WriteString(&buf, c.brand); err != nil {
					return err
				}
				brandPacket := &protocol.ServerboundCustomPayload{
					CustomPayloadData: protocol.CustomPayloadData{
						Channel: "minecraft:brand",
						Data:    buf.Bytes(),
					},
				}
				if err := c.WritePacket(brandPacket); err != nil {
					return fmt.Errorf("failed to send brand response: %w", err)
				}
			}

		case *protocol.ClientboundSelectKnownPacks:
			knownPacks := &protocol.ServerboundSelectKnownPacks{
				Packs: []protocol.KnownPack{},
			}
			if err := c.WritePacket(knownPacks); err != nil {
				return fmt.Errorf("failed to respond to known packs: %w", err)
			}
			if c.version >= protocol.V1_21_3 {
				finishConfig := &protocol.ServerboundFinishConfiguration{}
				if err := c.WritePacket(finishConfig); err != nil {
					return fmt.Errorf("failed to send finish configuration for 1.21+: %w", err)
				}
			}

		case *protocol.ClientboundFinishConfiguration:
			if c.version < protocol.V1_21_3 {
				finishConfig := &protocol.ServerboundFinishConfiguration{}
				if err := c.WritePacket(finishConfig); err != nil {
					return fmt.Errorf("failed to send final finish configuration: %w", err)
				}
			}
			c.SetState(protocol.StatePlay)
			return nil

		case *protocol.ClientboundDisconnect:
			return fmt.Errorf("disconnected during config: %s", p.Reason)

		case *protocol.ClientboundCookieRequest:
			cookieResp := &protocol.ServerboundCookieResponse{Key: p.Key, Data: nil}
			if err := c.WritePacket(cookieResp); err != nil {
				return fmt.Errorf("failed to respond to cookie request: %w", err)
			}
		case *protocol.ClientboundConfigPing,
			*protocol.ClientboundRegistryData,
			*protocol.ClientboundFeatureFlags,
			*protocol.ClientboundUpdateTags:

		default:
			log.Printf("[INFO] Ignoring unexpected packet during configuration: %T", p)
		}
	}
}

func (c *Client) ServerHostname() string {
	var serverHostname = c.serverHostname

	if serverHostname == "" {
		serverHostname = c.addr.IP.String()
	}

	return serverHostname
}

func (c *Client) SendHandshake(state protocol.State) error {
	if c.Conn == nil {
		return fmt.Errorf("client not connected")
	}

	handshake := &protocol.ServerboundHandshake{
		ProtocolVersion: c.version.Protocol(),
		ServerAddress:   c.ServerHostname(),
		ServerPort:      uint16(c.addr.Port),
		NextState:       state,
	}

	return c.WritePacket(handshake)
}

func (c *Client) SendLogin(username string, uniqueId uuid.UUID) error {
	if c.Conn == nil {
		return fmt.Errorf("client not connected")
	}

	if c.username == "" {
		return fmt.Errorf("invalid username")
	}

	if uniqueId == uuid.Nil && c.version >= protocol.V1_19_2 {
		uniqueId = protocol.OfflineUUID(c.username)
	}

	login := &protocol.ServerboundLoginStart{
		Username: username,
		UUID:     uniqueId,
	}

	return c.WritePacket(login)
}

func (c *Client) SendStatusRequest() error {
	if c.Conn == nil {
		return fmt.Errorf("client not connected")
	}

	return c.WritePacket(&protocol.ServerboundStatusRequest{})
}

func (c *Client) SendPingRequest() error {
	if c.Conn == nil {
		return fmt.Errorf("client not connected")
	}

	return c.WritePacket(&protocol.ServerboundPing{})
}

func (c *Client) SendClientSettings(settings protocol.ClientSettings) error {
	if c.Conn == nil {
		return fmt.Errorf("client not connected")
	}

	return c.WritePacket(&protocol.ServerboundClientSettings{
		ClientSettings: settings,
	})
}
