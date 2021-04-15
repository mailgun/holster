package udp

import (
	"fmt"
	"net"
	"time"
)

type Client struct {
	conn *net.UDPConn
	addr net.Addr
}

func NewClient(address string) (*Client, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("while resolving Upstream address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("while dialing udp '%s' - %w", addr.String(), err)
	}
	return &Client{
		conn: conn,
		addr: addr,
	}, nil
}

func (c *Client) Recv(b []byte, timeout time.Duration) (int, net.Addr, error) {
	deadline := time.Now().Add(timeout)
	if err := c.conn.SetReadDeadline(deadline); err != nil {
		return 0, nil, err
	}
	return c.conn.ReadFrom(b)
}

func (c *Client) Send(b []byte) (int, error) {
	return c.conn.Write(b)
}

func (c *Client) Close() {
	c.conn.Close()
}
