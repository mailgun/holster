package udp

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Proxy struct {
	conf     ProxyConfig
	listen   *Server
	upstream *Client
	mutex    sync.Mutex // ensure the handler isn't still executing when start()/stop() are called
}

type ProxyConfig struct {
	// The address:port we should bind too
	Listen string
	// The address:port of the upstream udp server we should forward requests too
	Upstream string
	// How long we should wait for a response from the upstream udp server
	UpstreamTimeout time.Duration
}

func NewProxy(conf ProxyConfig) *Proxy {
	return &Proxy{
		conf: conf,
	}
}

func (p *Proxy) Start() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.conf.Listen == "" {
		return errors.New("variable Listen cannot be empty")
	}

	if p.conf.Upstream == "" {
		return errors.New("variable Upstream cannot be empty")
	}

	var err error
	p.upstream, err = NewClient(p.conf.Upstream)
	if err != nil {
		return fmt.Errorf("while dialing upstream '%s' - %w", p.conf.Upstream, err)
	}

	p.listen, err = NewServer(ServerConfig{
		BindAddress: p.conf.Listen,
		Handler:     p.handler,
	})
	if err != nil {
		return fmt.Errorf("while attempting to listen on '%s' - %w", p.conf.Listen, err)
	}
	return nil
}

func (p *Proxy) handler(conn net.PacketConn, buf []byte, addr net.Addr) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	logrus.Debugf("proxy %d bytes %s -> %s", len(buf), addr.String(), p.upstream.addr.String())
	// Forward the request to upstream
	if _, err := p.upstream.Send(buf); err != nil {
		logrus.WithError(err).Errorf("failed to forward '%d' bytes from '%s' to upstream '%s'", len(buf), addr.String(), p.upstream.addr.String())
		return
	}

	// Wait for a response until timeout
	b := make([]byte, 10_000)
	n, _, _ := p.upstream.Recv(b, p.conf.UpstreamTimeout)

	// Nothing to send to upstream
	if n == 0 {
		return
	}

	// Send response to upstream
	if _, err := conn.WriteTo(b[:n], addr); err != nil {
		logrus.WithError(err).Errorf("failed to forward '%d' bytes from '%s' to downstream '%s'", n, p.upstream.addr.String(), addr.String())
		return
	}
	logrus.Debugf("proxy %d bytes %s <- %s", n, p.upstream.addr.String(), addr.String())
}

func (p *Proxy) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.listen.Close()
	p.upstream.Close()
}
