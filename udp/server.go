package udp

import (
	"fmt"
	"net"
	"strings"

	"github.com/sirupsen/logrus"
)

type Server struct {
	conn     net.PacketConn
	shutdown chan struct{}
}

type Handler func(net.PacketConn, []byte, net.Addr)

type ServerConfig struct {
	BindAddress string
	Handler     Handler
}

func NewServer(conf ServerConfig) (*Server, error) {
	conn, err := net.ListenPacket("udp", conf.BindAddress)
	if err != nil {
		return nil, fmt.Errorf("while listening on '%s' - %w", conf.BindAddress, err)
	}

	shutdown := make(chan struct{})
	logrus.Debugf("Listening [%s]...\n", conf.BindAddress)

	go func() {
		for {
			select {
			case <-shutdown:
				return
			default:
			}

			b := make([]byte, 10_000)
			n, addr, err := conn.ReadFrom(b)
			if err != nil {
				if strings.HasSuffix(err.Error(), "use of closed network connection") {
					return
				}
				logrus.WithError(err).Error("ReadFrom() failed")
				return
			}
			// fmt.Printf("packet-received: bytes=%d from=%s\n", n, addr.String())
			conf.Handler(conn, b[:n], addr)
		}
	}()
	return &Server{
		conn:     conn,
		shutdown: shutdown,
	}, nil
}

func (u *Server) Close() {
	close(u.shutdown)
	u.conn.Close()
}
