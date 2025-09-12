package protocol

import (
	"fmt"
	"reflect"
)

type Definition struct {
	ProtocolVersion int32
	PacketNames     map[State]map[Direction]map[int32]string
	PacketIDs       map[State]map[Direction]map[string]int32
}

var protocolRegistry = make(map[Version]*Definition)

func GetDefinition(v Version) *Definition {
	for current := v; current >= 0; current-- {
		if def, ok := protocolRegistry[current]; ok {
			if current != v {
				protocolRegistry[v] = def
			}
			return def
		}
	}
	return nil
}

func NewPacket(v Version, s State, d Direction, id int32) (Packet, error) {
	def := GetDefinition(v)
	if def == nil {
		return nil, fmt.Errorf("unknown protocol version %s", v)
	}

	name, ok := def.PacketNames[s][d][id]
	if !ok {
		return nil, ErrUnknownPacket
	}

	factory, ok := packetConstructors[name]
	if !ok {
		return nil, fmt.Errorf("no constructor for packet type '%s'", name)
	}
	return factory(), nil
}

func GetPacketID(v Version, s State, d Direction, p Packet) (int32, bool) {
	def := GetDefinition(v)
	if def == nil {
		return -1, false
	}

	packetType := reflect.TypeOf(p)
	name, ok := packetTypes[packetType]
	if !ok {
		return -1, false
	}

	id, ok := def.PacketIDs[s][d][name]
	return id, ok
}

func VersionFromString(s string) (Version, bool) {
	version, ok := stringToVersion[s]
	return version, ok
}
