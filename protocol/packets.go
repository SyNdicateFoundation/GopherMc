package protocol

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/SyNdicateFoundation/GopherMc/component"
	"github.com/SyNdicateFoundation/fastrand"
	"io"
	"math"
	"time"

	"github.com/google/uuid"
)

type Packet interface {
	Encode(w io.Writer, v Version) error
	Decode(r io.Reader, v Version) error
}

type ServerboundHandshake struct {
	ProtocolVersion int32
	ServerAddress   string
	ServerPort      uint16
	NextState       State
}

func (p *ServerboundHandshake) Encode(w io.Writer, _ Version) error {
	_ = WriteVarInt(w, p.ProtocolVersion)
	_ = WriteString(w, p.ServerAddress)
	_ = WriteUShort(w, p.ServerPort)
	return WriteVarInt(w, int32(p.NextState))
}
func (p *ServerboundHandshake) Decode(r io.Reader, _ Version) (err error) {
	p.ProtocolVersion, _ = ReadVarInt(r)
	p.ServerAddress, _ = ReadString(r)
	p.ServerPort, _ = ReadUShort(r)
	var ns int32
	ns, err = ReadVarInt(r)
	p.NextState = State(ns)
	return
}

type ServerboundStatusRequest struct{}

func (p *ServerboundStatusRequest) Encode(_ io.Writer, _ Version) error { return nil }
func (p *ServerboundStatusRequest) Decode(_ io.Reader, _ Version) error { return nil }

type ClientboundStatusResponse struct{ JSONResponse string }

func (p *ClientboundStatusResponse) Encode(w io.Writer, _ Version) error {
	return WriteString(w, p.JSONResponse)
}
func (p *ClientboundStatusResponse) Decode(r io.Reader, _ Version) (err error) {
	p.JSONResponse, err = ReadString(r)
	return
}

type ServerboundPing struct{ Payload int64 }

func (p *ServerboundPing) Encode(w io.Writer, _ Version) error { return WriteLong(w, p.Payload) }
func (p *ServerboundPing) Decode(r io.Reader, _ Version) (err error) {
	p.Payload, err = ReadLong(r)
	return
}

type ClientboundPong struct{ Payload int64 }

func (p *ClientboundPong) Encode(w io.Writer, _ Version) error { return WriteLong(w, p.Payload) }
func (p *ClientboundPong) Decode(r io.Reader, _ Version) (err error) {
	p.Payload, err = ReadLong(r)
	return
}

type ServerboundLoginStart struct {
	Username string
	UUID     uuid.UUID
}

func (p *ServerboundLoginStart) Encode(w io.Writer, v Version) error {
	if err := WriteString(w, p.Username); err != nil {
		return err
	}

	if v >= V1_19 && v <= V1_19_2 {
		if err := WriteBool(w, false); err != nil {
			return err
		}
	}

	if v >= V1_20_2 {
		if _, err := w.Write(p.UUID[:]); err != nil {
			return err
		}
	} else if v >= V1_19_2 && p.UUID != uuid.Nil {
		if err := WriteBool(w, true); err != nil {
			return err
		}

		_, err := w.Write(p.UUID[:])
		return err
	}

	return nil
}

func (p *ServerboundLoginStart) Decode(r io.Reader, _ Version) (err error) {
	p.Username, err = ReadString(r)
	return
}

type ServerboundLoginAcknowledged struct{}

func (p *ServerboundLoginAcknowledged) Encode(_ io.Writer, _ Version) error { return nil }
func (p *ServerboundLoginAcknowledged) Decode(_ io.Reader, _ Version) error { return nil }

type ClientboundSetCompression struct{ Threshold int32 }

func (p *ClientboundSetCompression) Encode(w io.Writer, _ Version) error {
	return WriteVarInt(w, p.Threshold)
}
func (p *ClientboundSetCompression) Decode(r io.Reader, _ Version) (err error) {
	p.Threshold, err = ReadVarInt(r)
	return
}

type ClientboundLoginSuccess struct {
	Username string
	UUID     uuid.UUID
}

func (p *ClientboundLoginSuccess) Encode(_ io.Writer, _ Version) error {
	panic("client should not send LoginSuccess")
}

func (p *ClientboundLoginSuccess) Decode(r io.Reader, v Version) (err error) {
	if v >= V1_16 {
		if p.UUID, err = ReadUUID(r); err != nil {
			return
		}
	} else {
		if p.UUID, err = ReadStringUUID(r); err != nil {
			return
		}
	}

	if p.Username, err = ReadString(r); err != nil {
		return
	}

	if v >= V1_19 {
		propCount, err := ReadVarInt(r)
		if err != nil {
			return err
		}

		for i := int32(0); i < propCount; i++ {
			if _, err := ReadString(r); err != nil {
				return err
			}

			if _, err := ReadString(r); err != nil {
				return err
			}

			hasSig, _ := ReadBool(r)
			if hasSig {
				if _, err := ReadString(r); err != nil {
					return err
				}
			}
		}
	}

	if v >= V1_20_5 && v <= V1_21_1 {
		if _, err := ReadBool(r); err != nil {
			return err
		}
	}

	return nil
}

type ServerboundFinishConfiguration struct{}

func (p *ServerboundFinishConfiguration) Encode(_ io.Writer, _ Version) error { return nil }
func (p *ServerboundFinishConfiguration) Decode(_ io.Reader, _ Version) error { return nil }

type ClientboundFinishConfiguration struct{}

func (p *ClientboundFinishConfiguration) Encode(_ io.Writer, _ Version) error { return nil }
func (p *ClientboundFinishConfiguration) Decode(_ io.Reader, _ Version) error { return nil }

type ClientboundConfigKeepAlive struct{ ID int64 }

func (p *ClientboundConfigKeepAlive) Encode(w io.Writer, _ Version) error { return WriteLong(w, p.ID) }
func (p *ClientboundConfigKeepAlive) Decode(r io.Reader, _ Version) (err error) {
	p.ID, err = ReadLong(r)
	return
}

type ServerboundConfigKeepAlive struct{ ID int64 }

func (p *ServerboundConfigKeepAlive) Encode(w io.Writer, _ Version) error { return WriteLong(w, p.ID) }
func (p *ServerboundConfigKeepAlive) Decode(r io.Reader, _ Version) (err error) {
	p.ID, err = ReadLong(r)
	return
}

type ServerboundSelectKnownPacks struct {
	Packs []KnownPack
}

func (p *ServerboundSelectKnownPacks) Encode(w io.Writer, _ Version) error {
	if err := WriteVarInt(w, int32(len(p.Packs))); err != nil {
		return err
	}

	for _, pk := range p.Packs {
		if err := WriteString(w, pk.Namespace); err != nil {
			return err
		}

		if err := WriteString(w, pk.ID); err != nil {
			return err
		}

		if err := WriteString(w, pk.Version); err != nil {
			return err
		}
	}
	return nil
}

func (p *ServerboundSelectKnownPacks) Decode(_ io.Reader, _ Version) error { return nil }

type ClientboundSelectKnownPacks struct {
	Packs []KnownPack
}

func (p *ClientboundSelectKnownPacks) Encode(_ io.Writer, _ Version) error {
	return nil
}

func (p *ClientboundSelectKnownPacks) Decode(r io.Reader, _ Version) error {
	count, err := ReadVarInt(r)
	if err != nil {
		return err
	}

	p.Packs = make([]KnownPack, count)

	for i := int32(0); i < count; i++ {
		p.Packs[i].Namespace, err = ReadString(r)
		if err != nil {
			return err
		}
		p.Packs[i].ID, err = ReadString(r)
		if err != nil {
			return err
		}
		p.Packs[i].Version, err = ReadString(r)
		if err != nil {
			return err
		}
	}

	return nil
}

type ClientboundCookieRequest struct {
	Key string
}

func (p *ClientboundCookieRequest) Encode(w io.Writer, _ Version) error {
	return WriteString(w, p.Key)
}

func (p *ClientboundCookieRequest) Decode(r io.Reader, _ Version) error {
	var err error
	p.Key, err = ReadString(r)
	return err
}

type ServerboundCookieResponse struct {
	Key     string
	HasData bool
	Data    []byte
}

func (p *ServerboundCookieResponse) Encode(w io.Writer, _ Version) error {
	_ = WriteString(w, p.Key)
	_ = WriteBool(w, p.HasData)
	if p.HasData {
		_ = WriteByteSlice(w, p.Data)
	}
	return nil
}

func (p *ServerboundCookieResponse) Decode(_ io.Reader, _ Version) error {
	return nil
}

type ClientboundConfigPing struct{ ID int32 }

func (p *ClientboundConfigPing) Encode(w io.Writer, _ Version) error {
	return binary.Write(w, binary.BigEndian, p.ID)
}
func (p *ClientboundConfigPing) Decode(r io.Reader, _ Version) error {
	return binary.Read(r, binary.BigEndian, &p.ID)
}

type ServerboundConfigPong struct{ ID int32 }

func (p *ServerboundConfigPong) Encode(w io.Writer, _ Version) error {
	return binary.Write(w, binary.BigEndian, p.ID)
}
func (p *ServerboundConfigPong) Decode(_ io.Reader, _ Version) error { return nil }

type ClientboundJoinGame struct{}

func (p *ClientboundJoinGame) Encode(_ io.Writer, _ Version) error {
	panic("client should not send JoinGame")
}

func (p *ClientboundJoinGame) Decode(r io.Reader, _ Version) error {
	_, err := io.Copy(io.Discard, r)
	return err
}

type ClientboundDisconnect struct{ Reason string }

func (p *ClientboundDisconnect) Encode(w io.Writer, _ Version) error {
	return WriteString(w, p.Reason)
}
func (p *ClientboundDisconnect) Decode(r io.Reader, _ Version) (err error) {
	p.Reason, err = ReadString(r)
	return
}

type ServerboundChatMessage struct {
	Message    string
	PrivateKey *rsa.PrivateKey
	UUID       uuid.UUID
}

func (p *ServerboundChatMessage) Encode(w io.Writer, v Version) error {
	if v < V1_19 {
		return WriteString(w, p.Message)
	}

	timestamp := time.Now().UnixMilli()
	salt := fastrand.NumberN[int64](math.MaxInt64)

	signature, err := p.signMessage(v, p.Message, timestamp, salt)
	if err != nil {
		return err
	}

	_ = WriteString(w, p.Message)
	_ = WriteLong(w, timestamp)
	_ = WriteLong(w, salt)

	if v >= V1_19_3 {
		if signature != nil {
			_ = WriteBool(w, true)
			_, _ = w.Write(signature)
		} else {
			_ = WriteBool(w, false)
		}

		_ = WriteVarInt(w, 0)
		_, _ = w.Write(make([]byte, 3))

		if v >= V1_21_5 {
			_ = WriteByte(w, 0)
		}
	} else if v >= V1_19_2 {
		_ = WriteByteSlice(w, signature)
		_ = WriteBool(w, false)
		_ = WriteVarInt(w, 0)
		_ = WriteBool(w, false)
	} else {
		_ = WriteByteSlice(w, signature)
		_ = WriteBool(w, false)
	}

	return nil
}

func (p *ServerboundChatMessage) signMessage(v Version, message string, timestamp int64, salt int64) ([]byte, error) {
	if p.PrivateKey == nil {
		return nil, nil
	}

	var signBuf []byte
	if v >= V1_19_2 {
		signBuf = make([]byte, 8+8+16+len(message))
		binary.BigEndian.PutUint64(signBuf[0:8], uint64(salt))
		binary.BigEndian.PutUint64(signBuf[8:16], uint64(timestamp))
		copy(signBuf[16:32], p.UUID[:])
		copy(signBuf[32:], message)
	} else {
		signBuf = make([]byte, 8+8+len(message))
		binary.BigEndian.PutUint64(signBuf[0:8], uint64(timestamp))
		binary.BigEndian.PutUint64(signBuf[8:16], uint64(salt))
		copy(signBuf[16:], message)
	}

	hasher := sha256.New()
	hasher.Write(signBuf)
	hash := hasher.Sum(nil)

	signature, err := rsa.SignPKCS1v15(fastrand.FastReader, p.PrivateKey, crypto.SHA256, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to sign chat message: %w", err)
	}

	if v >= V1_19_3 {
		signed := make([]byte, 256)
		copy(signed, signature)
		return signed, nil
	}

	return signature, nil
}

func (p *ServerboundChatMessage) Decode(r io.Reader, _ Version) (err error) {
	p.Message, err = ReadString(r)
	return
}

type ClientboundKeepAlive struct{ ID int64 }

func (p *ClientboundKeepAlive) Encode(_ io.Writer, _ Version) error {
	panic("client should not send ClientboundKeepAlive")
}
func (p *ClientboundKeepAlive) Decode(r io.Reader, v Version) (err error) {
	if v >= V1_12_2 {
		p.ID, err = ReadLong(r)
	} else {
		var id int32
		id, err = ReadVarInt(r)
		p.ID = int64(id)
	}
	return
}

type ServerboundKeepAlive struct{ ID int64 }

func (p *ServerboundKeepAlive) Encode(w io.Writer, v Version) error {
	if v >= V1_12_2 {
		return WriteLong(w, p.ID)
	}
	return WriteVarInt(w, int32(p.ID))
}
func (p *ServerboundKeepAlive) Decode(_ io.Reader, _ Version) error {
	panic("server should not decode ServerboundKeepAlive")
}

type ServerboundClientSettings struct {
	ClientSettings
}

func (p *ServerboundClientSettings) Encode(w io.Writer, v Version) error {
	if err := WriteString(w, p.Locale); err != nil {
		return err
	}

	if err := WriteByte(w, p.View); err != nil {
		return err
	}

	if v <= V1_7 {
		if err := WriteByte(w, 0x1); err != nil {
			return err
		}
	} else if v >= V1_8 {
		if err := WriteVarInt(w, p.ChatMode); err != nil {
			return err
		}
	}

	if err := WriteBool(w, p.ChatColors); err != nil {
		return err
	}

	if v <= V1_7 {
		if err := WriteByte(w, p.SkinParts&0x01); err != nil {
			return err
		}
		if err := WriteBool(w, true); err != nil {
			return err
		}
	}

	if v >= V1_8 {
		if err := WriteByte(w, p.SkinParts); err != nil {
			return err
		}
	}

	if v >= V1_9 {
		if err := WriteVarInt(w, p.MainHand); err != nil {
			return err
		}
	}

	if v >= V1_17_1 {
		if err := WriteBool(w, false); err != nil {
			return err
		}
	}

	if v >= V1_18 {
		if err := WriteBool(w, true); err != nil {
			return err
		}
	}

	if v >= V1_21_3 {
		if err := WriteVarInt(w, 0); err != nil {
			return err
		}
	}

	return nil
}

func (p *ServerboundClientSettings) Decode(_ io.Reader, _ Version) error { return nil }

type CustomPayload struct {
	Channel string
	Data    []byte
}

func (p *CustomPayload) Encode(w io.Writer, _ Version) error {
	_ = WriteString(w, p.Channel)
	_, err := w.Write(p.Data)
	return err
}
func (p *CustomPayload) Decode(r io.Reader, _ Version) (err error) {
	p.Channel, err = ReadString(r)
	if err != nil {
		return err
	}
	p.Data, err = io.ReadAll(r)
	return err
}

type CustomPayloadData struct {
	Channel string
	Data    []byte
}

func (p *CustomPayloadData) encode(w io.Writer, _ Version) error {
	_ = WriteString(w, p.Channel)
	_, err := w.Write(p.Data)
	return err
}

func (p *CustomPayloadData) decode(r io.Reader, _ Version) (err error) {
	p.Channel, err = ReadString(r)
	if err != nil {
		return err
	}

	p.Data, err = io.ReadAll(r)
	return err
}

type ServerboundCustomPayload struct{ CustomPayloadData }

func (p *ServerboundCustomPayload) Encode(w io.Writer, v Version) error { return p.encode(w, v) }
func (p *ServerboundCustomPayload) Decode(r io.Reader, v Version) error { return p.decode(r, v) }

type ClientboundCustomPayload struct{ CustomPayloadData }

func (p *ClientboundCustomPayload) Encode(w io.Writer, v Version) error { return p.encode(w, v) }
func (p *ClientboundCustomPayload) Decode(r io.Reader, v Version) error { return p.decode(r, v) }

type ClientboundFeatureFlags struct {
	Features []string
}

func (p *ClientboundFeatureFlags) Encode(_ io.Writer, _ Version) error {
	panic("client should not send FeatureFlags")
}

func (p *ClientboundFeatureFlags) Decode(r io.Reader, _ Version) error {
	count, err := ReadVarInt(r)
	if err != nil {
		return err
	}

	p.Features = make([]string, count)
	for i := 0; i < int(count); i++ {
		p.Features[i], err = ReadString(r)
		if err != nil {
			return err
		}
	}

	return nil
}

type ClientboundUpdateTags struct {
	Tags []RegistryTag
}

type RegistryTag struct {
	Registry string
	Tags     []Tag
}

type Tag struct {
	Name    string
	Entries []int32
}

func (p *ClientboundUpdateTags) Encode(_ io.Writer, _ Version) error {
	panic("client should not send UpdateTags")
}

func (p *ClientboundUpdateTags) Decode(r io.Reader, _ Version) error {
	count, err := ReadVarInt(r)
	if err != nil {
		return err
	}

	p.Tags = make([]RegistryTag, count)

	for i := 0; i < int(count); i++ {
		p.Tags[i].Registry, err = ReadString(r)
		if err != nil {
			return err
		}

		tagCount, err := ReadVarInt(r)
		if err != nil {
			return err
		}

		p.Tags[i].Tags = make([]Tag, tagCount)

		for j := 0; j < int(tagCount); j++ {
			p.Tags[i].Tags[j].Name, err = ReadString(r)
			if err != nil {
				return err
			}

			entryCount, err := ReadVarInt(r)
			if err != nil {
				return err
			}

			p.Tags[i].Tags[j].Entries = make([]int32, entryCount)
			for k := 0; k < int(entryCount); k++ {
				p.Tags[i].Tags[j].Entries[k], err = ReadVarInt(r)

				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type ClientboundRegistryData struct {
	Registry string
	Data     []byte
}

func (p *ClientboundRegistryData) Encode(_ io.Writer, _ Version) error {
	panic("client should not send RegistryData")
}

func (p *ClientboundRegistryData) Decode(r io.Reader, _ Version) error {
	p.Registry, _ = ReadString(r)
	p.Data, _ = ReadBytes(r)
	return nil
}

type ClientboundChatMessage struct {
	Component component.ChatComponent
	Sender    string
}

func (p *ClientboundChatMessage) Encode(_ io.Writer, _ Version) error {
	return errors.New("client should not send Clientbound ChatMessage")
}

func (p *ClientboundChatMessage) Decode(r io.Reader, v Version) (err error) {
	//chatData, err := ReadString(r)
	//if err != nil {
	//	return err
	//}
	//
	//var chatComponent component.ChatComponent
	//
	// TODO code it

	return nil
}

type ServerboundPlayerPosition struct {
	X, Y, Z  float64
	OnGround bool
}

func (p *ServerboundPlayerPosition) Encode(w io.Writer, _ Version) error {
	_ = binary.Write(w, binary.BigEndian, p.X)
	_ = binary.Write(w, binary.BigEndian, p.Y)
	_ = binary.Write(w, binary.BigEndian, p.Z)
	// TODO code it
	return WriteBool(w, p.OnGround)
}

func (p *ServerboundPlayerPosition) Decode(_ io.Reader, _ Version) error {
	return errors.New("server should not receive ServerboundPlayerPosition")
}
