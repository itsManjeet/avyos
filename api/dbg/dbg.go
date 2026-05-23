package dbg

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"avyos.dev/pkg/sutra"
)

const (
	ServiceName    = "dev.avyos.dbg"
	DefaultTCPPort = 5037
)

// NewTCPClient dials the dbg service over TCP.
func NewTCPClient(address string) (*Client, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(DefaultTCPPort))
	}
	nc, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	conn := sutra.NewConn(nc)
	return &Client{
		conn: conn,
		Dbg:  NewDbgClient(conn, 0),
	}, nil
}

// NewHostClient dials the dbg service via host:port.
func NewHostClient(host string, port int) (*Client, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port: %d", port)
	}
	return NewTCPClient(net.JoinHostPort(host, strconv.Itoa(port)))
}

// Raw returns the underlying sutra connection.
func (c *Client) Raw() *sutra.Conn {
	return c.conn
}
