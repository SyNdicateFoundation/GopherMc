//go:generate go run ../generator

package protocol

import (
	"crypto/sha1"
	"github.com/google/uuid"
	"sync"
)

type State int

const (
	StateHandshaking State = iota
	StateStatus
	StateLogin
	StatePlay
	StateConfiguration
)

func (s State) String() string {
	switch s {
	case StateHandshaking:
		return "Handshake"
	case StateStatus:
		return "Status"
	case StateLogin:
		return "Login"
	case StateConfiguration:
		return "Configuration"
	case StatePlay:
		return "Play"
	default:
		return "Unknown"
	}
}

type Direction bool

const (
	DirectionServerbound Direction = true
	DirectionClientbound Direction = false
)

func OfflineUUID(username string) uuid.UUID {
	hasher := sha1.New()
	hasher.Write([]byte("OfflinePlayer:" + username))
	hash := hasher.Sum(nil)
	var offlineUUID uuid.UUID
	copy(offlineUUID[:], hash)
	offlineUUID[6] = (offlineUUID[6] & 0x0f) | 0x30
	offlineUUID[8] = (offlineUUID[8] & 0x3f) | 0x80
	return offlineUUID
}

type ClientSettings struct {
	Locale     string
	View       byte
	ChatMode   int32
	ChatColors bool
	SkinParts  byte
	MainHand   int32
}

type KnownPack struct {
	Namespace string
	ID        string
	Version   string
}

type PlayerPosition struct {
	mu                  sync.RWMutex
	X, Y, Z             float64
	Yaw, HeadYaw, Pitch float32
	OnGround            bool
}

func (p *PlayerPosition) Update(
	x float64,
	y float64,
	z float64,
	yaw float32,
	headYaw float32,
	pitch float32,
	ground bool,
) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.X, p.Y, p.Z = x, y, z
	p.Yaw, p.HeadYaw, p.Pitch = yaw, headYaw, pitch
	p.OnGround = ground
}
