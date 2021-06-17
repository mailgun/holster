package udp_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/mailgun/holster/v4/udp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerClient(t *testing.T) {
	srv, err := udp.NewServer(udp.ServerConfig{
		BindAddress: "127.0.0.1:5001",
		Handler: func(conn net.PacketConn, recv []byte, addr net.Addr) {
			resp := fmt.Sprintf("Hello, %s", string(recv))
			_, err := conn.WriteTo([]byte(resp), addr)
			require.NoError(t, err)
		},
	})
	require.NoError(t, err)
	defer srv.Close()

	conn, err := udp.NewClient("127.0.0.1:5001")
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Send([]byte("Thrawn"))
	require.NoError(t, err)

	b := make([]byte, 50)
	n, _, err := conn.Recv(b, time.Second)

	assert.Equal(t, "Hello, Thrawn", string(b[:n]))
}

func TestProxy(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	srv, err := udp.NewServer(udp.ServerConfig{
		BindAddress: "127.0.0.1:5001",
		Handler: func(conn net.PacketConn, recv []byte, addr net.Addr) {
			resp := fmt.Sprintf("Hello, %s", string(recv))
			_, err := conn.WriteTo([]byte(resp), addr)
			require.NoError(t, err)
		},
	})
	require.NoError(t, err)
	defer srv.Close()

	p := udp.NewProxy(udp.ProxyConfig{
		Listen:          "127.0.0.1:5000",
		Upstream:        "127.0.0.1:5001",
		UpstreamTimeout: time.Second,
	})

	// Start the proxy for our udp server
	err = p.Start()
	require.NoError(t, err)
	defer p.Stop()

	conn, err := udp.NewClient("127.0.0.1:5000")
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Send([]byte("Admiral"))
	require.NoError(t, err)

	b := make([]byte, 50)
	n, _, err := conn.Recv(b, time.Second)
	require.NoError(t, err)

	assert.Equal(t, "Hello, Admiral", string(b[:n]))

	// Shutdown the proxy
	p.Stop()

	// Should not get a response from the upstream server
	_, err = conn.Send([]byte("Not expecting a response"))
	require.NoError(t, err)
	n, _, err = conn.Recv(b, time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recvfrom: connection refused")

	// Start the proxy again
	p.Start()

	// Should get a response
	_, err = conn.Send([]byte("World"))
	require.NoError(t, err)

	n, _, err = conn.Recv(b, time.Second)
	require.NoError(t, err)

	assert.Equal(t, "Hello, World", string(b[:n]))

}
