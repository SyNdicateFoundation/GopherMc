package protocol

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/valyala/bytebufferpool"
)

const debugMinecraft = false

type Conn struct {
	net.Conn

	version Version
	state   State

	compressionThreshold int

	readerLock sync.Mutex
	writerLock sync.Mutex

	zlibReader io.ReadCloser
}

func NewConn(conn net.Conn, version Version) *Conn {
	return &Conn{
		Conn:                 conn,
		version:              version,
		state:                StateHandshaking,
		compressionThreshold: -1,
	}
}

func (c *Conn) State() State {
	return c.state
}

func (c *Conn) SetState(s State) {
	c.state = s
}

func (c *Conn) SetCompression(threshold int) {
	c.compressionThreshold = threshold
}

var ErrUnknownPacket = errors.New("unknown packet")

func (c *Conn) ReadPacket() (Packet, error) {
	c.readerLock.Lock()
	defer c.readerLock.Unlock()

	packetLength, err := ReadVarInt(c.Conn)
	if err != nil {
		return nil, fmt.Errorf("read packet length: %w", err)
	}

	packetBuffer := bytebufferpool.Get()
	defer bytebufferpool.Put(packetBuffer)

	if _, err := io.CopyN(packetBuffer, c.Conn, int64(packetLength)); err != nil {
		return nil, fmt.Errorf("read packet data: %w", err)
	}

	packetReader := bytes.NewReader(packetBuffer.B)

	var dataReader io.Reader
	if c.compressionThreshold >= 0 {
		dataLength, err := ReadVarInt(packetReader)
		if err != nil {
			return nil, fmt.Errorf("read compressed data length: %w", err)
		}
		if dataLength == 0 {
			dataReader = packetReader
		} else {
			if c.zlibReader == nil {
				c.zlibReader, err = zlib.NewReader(packetReader)
				if err != nil {
					return nil, fmt.Errorf("create zlib reader: %w", err)
				}
			} else {
				if err = c.zlibReader.(zlib.Resetter).Reset(packetReader, nil); err != nil {
					return nil, fmt.Errorf("reset zlib reader: %w", err)
				}
			}
			dataReader = c.zlibReader
		}
	} else {
		dataReader = packetReader
	}

	packetID, err := ReadVarInt(dataReader)
	if err != nil {
		return nil, fmt.Errorf("read packet ID: %w", err)
	}

	packet, err := NewPacket(c.version, c.state, DirectionClientbound, packetID)
	if err != nil {
		_, _ = io.Copy(io.Discard, dataReader)
		if debugMinecraft {
			log.Printf("[DEBUG] S -> C | State: %-12s | ID: 0x%02X | Type: %s (IGNORED)", c.state, packetID, "Unknown")
		}
		return nil, ErrUnknownPacket
	}

	if err := packet.Decode(dataReader, c.version); err != nil {
		return nil, fmt.Errorf("decode packet 0x%02X (%T): %w", packetID, packet, err)
	}

	if debugMinecraft {
		log.Printf("[DEBUG] S -> C | State: %-12s | ID: 0x%02X | Type: %T", c.state, packetID, packet)
	}

	return packet, nil
}

func (c *Conn) WritePacket(p Packet) error {
	c.writerLock.Lock()
	defer c.writerLock.Unlock()

	packetID, ok := GetPacketID(c.version, c.state, DirectionServerbound, p)
	if !ok {
		return fmt.Errorf("no id for packet %T in state %s (version %s)", p, c.state, c.version)
	}

	dataBuf := bytebufferpool.Get()
	defer bytebufferpool.Put(dataBuf)

	_ = WriteVarInt(dataBuf, packetID)
	if err := p.Encode(dataBuf, c.version); err != nil {
		return fmt.Errorf("encode packet data for %T: %w", p, err)
	}

	finalPayload := bytebufferpool.Get()
	defer bytebufferpool.Put(finalPayload)

	if c.compressionThreshold >= 0 {
		if dataBuf.Len() >= c.compressionThreshold {
			_ = WriteVarInt(finalPayload, int32(dataBuf.Len()))
			zWriter := zlib.NewWriter(finalPayload)
			_, _ = zWriter.Write(dataBuf.B)
			_ = zWriter.Close()
		} else {
			_ = WriteVarInt(finalPayload, 0)
			_, _ = finalPayload.Write(dataBuf.B)
		}
	} else {
		_, _ = finalPayload.Write(dataBuf.B)
	}

	_ = WriteVarInt(c.Conn, int32(finalPayload.Len()))
	if _, err := c.Conn.Write(finalPayload.B); err != nil {
		return fmt.Errorf("write final payload to network: %w", err)
	}

	if debugMinecraft {
		log.Printf("[DEBUG] C -> S | State: %-12s | ID: 0x%02X | Type: %T", c.state, packetID, p)
	}

	return nil
}

func (c *Conn) Close() error {
	if c.zlibReader != nil {
		_ = c.zlibReader.Close()
	}
	return c.Conn.Close()
}
