package protocol

import "reflect"

type PacketFactory func() Packet

var packetConstructors = map[string]PacketFactory{
	"ServerboundHandshake":  func() Packet { return &ServerboundHandshake{} },
	"ClientboundDisconnect": func() Packet { return &ClientboundDisconnect{} },

	"ServerboundStatusRequest":  func() Packet { return &ServerboundStatusRequest{} },
	"ClientboundStatusResponse": func() Packet { return &ClientboundStatusResponse{} },
	"ServerboundPing":           func() Packet { return &ServerboundPing{} },
	"ClientboundPong":           func() Packet { return &ClientboundPong{} },

	"ServerboundLoginStart":        func() Packet { return &ServerboundLoginStart{} },
	"ClientboundLoginSuccess":      func() Packet { return &ClientboundLoginSuccess{} },
	"ClientboundSetCompression":    func() Packet { return &ClientboundSetCompression{} },
	"ServerboundLoginAcknowledged": func() Packet { return &ServerboundLoginAcknowledged{} },

	"ServerboundFinishConfiguration": func() Packet { return &ServerboundFinishConfiguration{} },
	"ClientboundFinishConfiguration": func() Packet { return &ClientboundFinishConfiguration{} },
	"ServerboundConfigKeepAlive":     func() Packet { return &ServerboundConfigKeepAlive{} },
	"ClientboundConfigKeepAlive":     func() Packet { return &ClientboundConfigKeepAlive{} },
	"ServerboundSelectKnownPacks":    func() Packet { return &ServerboundSelectKnownPacks{} },
	"ClientboundSelectKnownPacks":    func() Packet { return &ClientboundSelectKnownPacks{} },
	"ClientboundCookieRequest":       func() Packet { return &ClientboundCookieRequest{} },
	"ServerboundCookieResponse":      func() Packet { return &ServerboundCookieResponse{} },
	"ClientboundConfigPing":          func() Packet { return &ClientboundConfigPing{} },
	"ServerboundConfigPong":          func() Packet { return &ServerboundConfigPong{} },

	"ServerboundChatMessage": func() Packet { return &ServerboundChatMessage{} },
	"ClientboundKeepAlive":   func() Packet { return &ClientboundKeepAlive{} },
	"ServerboundKeepAlive":   func() Packet { return &ServerboundKeepAlive{} },
	"ClientboundJoinGame":    func() Packet { return &ClientboundJoinGame{} },

	"ServerboundClientSettings": func() Packet { return &ServerboundClientSettings{} },
	"ServerboundCustomPayload":  func() Packet { return &ServerboundCustomPayload{} },
	"ClientboundCustomPayload":  func() Packet { return &ClientboundCustomPayload{} },
	"ClientboundFeatureFlags":   func() Packet { return &ClientboundFeatureFlags{} },
	"ClientboundUpdateTags":     func() Packet { return &ClientboundUpdateTags{} },
	"ClientboundRegistryData":   func() Packet { return &ClientboundRegistryData{} },
}

var packetTypes = make(map[reflect.Type]string)

func init() {
	for name, factory := range packetConstructors {
		packetTypes[reflect.TypeOf(factory())] = name
	}
}
