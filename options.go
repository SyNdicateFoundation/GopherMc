package gophermc

import (
	"crypto/rsa"
	"github.com/SyNdicateFoundation/GopherMc/protocol"
	"github.com/google/uuid"
	"net"
)

type ClientOption func(*Client)

func WithVersion(version protocol.Version) ClientOption {
	return func(c *Client) {
		c.version = version
	}
}

func WithTCPAddr(tcpAddr *net.TCPAddr) ClientOption {
	return func(c *Client) {
		c.addr = tcpAddr
	}
}

func WithAddr(addr string) ClientOption {
	return func(c *Client) {
		host, port, _ := net.SplitHostPort(addr)
		if host == "" {
			host = addr
		}

		WithServerHostname(host)(c)

		if port == "" {
			addr = net.JoinHostPort(addr, "25565")
		}

		tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return
		}

		WithTCPAddr(tcpAddr)(c)
	}
}

func WithServerHostname(serverHostname string) ClientOption {
	return func(c *Client) {
		c.serverHostname = serverHostname
	}
}

func WithBrand(brand string) ClientOption {
	return func(c *Client) {
		c.brand = brand
	}
}

func WithPrivateKey(key *rsa.PrivateKey) ClientOption {
	return func(c *Client) {
		c.privateKey = key
	}
}

func WithUsername(username string) ClientOption {
	return func(c *Client) {
		c.username = username
		c.offlineUuid = protocol.OfflineUUID(username)
	}
}
func WithUUID(uuid uuid.UUID) ClientOption {
	return func(c *Client) {
		c.uniqueId = uuid
	}
}
