package protocol

import (
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	"io"
)

const (
	MaxVarIntSize     = 5
	MaxPacketDataSize = 2097152
)

func ReadUUID(r io.Reader) (uuid.UUID, error) {
	var b [16]byte
	_, err := io.ReadFull(r, b[:])
	return b, err
}

func ReadStringUUID(r io.Reader) (uuid.UUID, error) {
	var s string
	var err error

	if s, err = ReadString(r); err != nil {
		return uuid.Nil, err
	}

	if len(s) == 32 {
		s = fmt.Sprintf("%s-%s-%s-%s-%s", s[0:8], s[8:12], s[12:16], s[16:20], s[20:32])
	}

	return uuid.Parse(s)
}
func WriteVarInt(w io.Writer, value int32) error {
	uv := uint32(value)
	for {
		if (uv & ^uint32(0x7F)) == 0 {
			return WriteByte(w, byte(uv))
		}
		if err := WriteByte(w, byte(uv&0x7F|0x80)); err != nil {
			return err
		}
		uv >>= 7
	}
}
func ReadVarInt(r io.Reader) (int32, error) {
	var val uint32
	var pos uint
	for i := 0; i < MaxVarIntSize; i++ {
		b, err := ReadByte(r)
		if err != nil {
			return 0, err
		}
		val |= uint32(b&0x7F) << pos
		if (b & 0x80) == 0 {
			return int32(val), nil
		}
		pos += 7
	}
	return 0, fmt.Errorf("varint is too big")
}
func WriteString(w io.Writer, value string) error {
	return WriteByteSlice(w, []byte(value))
}
func ReadString(r io.Reader) (string, error) {
	data, err := ReadBytes(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ReadBytes(r io.Reader) ([]byte, error) {
	length, err := ReadVarInt(r)
	if err != nil {
		return nil, fmt.Errorf("read byte array length: %w", err)
	}
	if length < 0 {
		return nil, fmt.Errorf("byte array length is negative: %d", length)
	}
	if length > MaxPacketDataSize {
		return nil, fmt.Errorf("byte array length %d exceeds max size", length)
	}
	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	return buf, err
}

func WriteByteSlice(w io.Writer, data []byte) error {
	if err := WriteVarInt(w, int32(len(data))); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func ReadByte(r io.Reader) (byte, error) {
	if br, ok := r.(io.ByteReader); ok {
		return br.ReadByte()
	}
	var b [1]byte
	_, err := io.ReadFull(r, b[:])
	return b[0], err
}

func WriteByte(w io.Writer, b byte) error {
	if bw, ok := w.(io.ByteWriter); ok {
		return bw.WriteByte(b)
	}
	_, err := w.Write([]byte{b})
	return err
}

func ReadLong(r io.Reader) (int64, error) {
	var v int64
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

func WriteLong(w io.Writer, v int64) error {
	return binary.Write(w, binary.BigEndian, v)
}

func ReadUShort(r io.Reader) (uint16, error) {
	var v uint16
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

func WriteUShort(w io.Writer, v uint16) error {
	return binary.Write(w, binary.BigEndian, v)
}

func ReadBool(r io.Reader) (bool, error) {
	b, err := ReadByte(r)
	return b != 0, err
}

func WriteBool(w io.Writer, v bool) error {
	var b byte = 0
	if v {
		b = 1
	}
	return WriteByte(w, b)
}
