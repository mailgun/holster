package grpcconn

// gRPC connection pooling

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/mailgun/errors"
	"github.com/mailgun/holster/v4/clock"
	"github.com/mailgun/holster/v4/setter"
	"github.com/mailgun/holster/v4/tracing"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	grpcCodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcStatus "google.golang.org/grpc/status"
)

const (
	minConnectionTTL        = 10 * clock.Second
	defaultConnPoolCapacity = 16
	defaultNumConnections   = 1
)

var (
	ErrConnMgrClosed = errors.New("connection manager closed")
	errConnPoolEmpty = errors.New("connection pool empty")

	MetricGRPCConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "grpcconn_connections",
		Help: "The number of gRPC connections used by grpcconn.",
	}, []string{"remote_service", "peer"})
)

type Config struct {
	RPCTimeout         clock.Duration
	BackOffTimeout     clock.Duration
	Zone               string
	OverrideHostHeader string

	// NumConnections is the number of client connections to establish
	// per target endpoint
	//
	// NOTE: A single GRPC client opens a maximum of 100 HTTP/2 Connections
	// to an endpoint. Once those connections are saturated, it will queue
	// requests to be delivered once there is availability. The recommended
	// method of overcoming this limitation is establishing multiple GPRC client
	// connections. See https://grpc.io/docs/guides/performance/
	NumConnections int
}

// ConnFactory creates gRPC client objects.
type ConnFactory[T any] interface {
	NewGRPCClient(cc grpc.ClientConnInterface) T
	GetServerListURL() string
	ServiceName() string
	ShouldDisposeOfConn(err error) bool
}

// ConnMgr automates gRPC `Connection` pooling.  This is necessary for use
// cases requiring frequent stream creation and high stream concurrency to
// avoid reaching the default 100 stream per connection limit.
// ConnMgr resolves gRPC instance endpoints and connects to them. Both
// resolution and connection is performed on the background allowing any number
// of concurrent AcquireConn to result in only one reconnect event.
type ConnMgr[T any] struct {
	cfg             *Config
	getEndpointsURL string
	connFactory     ConnFactory[T]
	httpClt         *http.Client
	reconnectCh     chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
	closeWG         sync.WaitGroup
	idPool          *IDPool

	connPoolMu    sync.RWMutex
	connPool      []*Conn[T]
	nextConnPivot uint64
	connectedCh   chan struct{}
}

type Conn[T any] struct {
	inner   *grpc.ClientConn
	client  T
	counter int
	broken  bool
	id      ID
}

func (c *Conn[T]) Client() T      { return c.client }
func (c *Conn[T]) Target() string { return c.inner.Target() }
func (c *Conn[T]) ID() string     { return c.id.String() }

// NewConnMgr instantiates a connection manager that maintains a gRPC
// connection pool.
func NewConnMgr[T any](cfg *Config, httpClient *http.Client, connFactory ConnFactory[T]) *ConnMgr[T] {
	// This ensures NumConnections is always at least 1
	setter.SetDefault(&cfg.NumConnections, defaultNumConnections)
	gc := ConnMgr[T]{
		cfg:             cfg,
		getEndpointsURL: connFactory.GetServerListURL() + "?zone=" + cfg.Zone,
		connFactory:     connFactory,
		httpClt:         httpClient,
		reconnectCh:     make(chan struct{}, 1),
		connPool:        make([]*Conn[T], 0, defaultConnPoolCapacity),
		idPool:          NewIDPool(),
	}
	gc.ctx, gc.cancel = context.WithCancel(context.Background())
	gc.closeWG.Add(1)
	go gc.run()
	return &gc
}

func (cm *ConnMgr[T]) AcquireConn(ctx context.Context) (_ *Conn[T], err error) {
	ctx = tracing.StartScope(ctx)
	defer func() {
		tracing.EndScope(ctx, err)
	}()

	for {
		// If the connection manager is already closed then return an error.
		if cm.ctx.Err() != nil {
			return nil, ErrConnMgrClosed
		}
		cm.connPoolMu.Lock()
		// Increment the connection index pivot to ensure that we select a
		// different connection every time when the load distribution is even.
		cm.nextConnPivot++
		// Select the least loaded connection.
		connPoolSize := len(cm.connPool)
		var leastLoadedConn *Conn[T]
		if connPoolSize > 0 {
			currConnIdx := cm.nextConnPivot % uint64(connPoolSize)
			leastLoadedConn = cm.connPool[currConnIdx]
			for i := 1; i < connPoolSize; i++ {
				currConnIdx = (cm.nextConnPivot + uint64(i)) % uint64(connPoolSize)
				currConn := cm.connPool[currConnIdx]
				if currConn.counter < leastLoadedConn.counter {
					leastLoadedConn = currConn
				}
			}
		}
		// If a least loaded connection is selected, then return it.
		if leastLoadedConn != nil {
			leastLoadedConn.counter++
			cm.connPoolMu.Unlock()
			return leastLoadedConn, nil
		}
		// We have got nothing to offer, let's refresh the connection pool to
		// get more connections.
		connectedCh := cm.ensureReconnect()
		cm.connPoolMu.Unlock()
		// Wait for the connection pool to be refreshed on the background or
		// the operation timeout elapsing.
		select {
		case <-connectedCh:
			continue
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (cm *ConnMgr[T]) ensureReconnect() chan struct{} {
	if cm.connectedCh != nil {
		return cm.connectedCh
	}
	cm.connectedCh = make(chan struct{})
	select {
	case cm.reconnectCh <- struct{}{}:
	default:
	}
	return cm.connectedCh
}

func (cm *ConnMgr[T]) ReleaseConn(conn *Conn[T], err error) bool {
	cm.connPoolMu.Lock()
	removedFromPool := false
	connPoolSize := len(cm.connPool)
	if cm.shouldDisposeOfConn(conn, err) {
		conn.broken = true
		// Remove the connection from the pool.
		for i, currConn := range cm.connPool {
			if currConn != conn {
				continue
			}
			copy(cm.connPool[i:], cm.connPool[i+1:])
			lastIdx := len(cm.connPool) - 1
			cm.connPool[lastIdx] = nil
			cm.connPool = cm.connPool[:lastIdx]
			removedFromPool = true
			connPoolSize = len(cm.connPool)
			cm.idPool.Release(conn.id)
			MetricGRPCConnections.WithLabelValues(cm.connFactory.ServiceName(), conn.Target()).Dec()
			break
		}
		cm.ensureReconnect()
	}
	conn.counter--
	closeConn := false
	if conn.broken && conn.counter < 1 {
		closeConn = true
	}
	cm.connPoolMu.Unlock()

	if removedFromPool {
		logrus.WithError(err).Warnf("Server removed from %s pool: %s, poolSize=%d, reason=%s",
			cm.connFactory.ServiceName(), conn.Target(), connPoolSize, err)
	}
	if closeConn {
		_ = conn.inner.Close()
		logrus.Warnf("Disconnected from %s server %s", cm.connFactory.ServiceName(), conn.Target())
		return true
	}
	return false
}

func (cm *ConnMgr[T]) shouldDisposeOfConn(conn *Conn[T], err error) bool {
	if conn.broken {
		return false
	}
	if err == nil {
		return false
	}

	rootErr := errors.Cause(err)
	if errors.Is(rootErr, context.Canceled) {
		return false
	}
	if errors.Is(rootErr, context.DeadlineExceeded) {
		return false
	}
	switch grpcStatus.Code(err) {
	case grpcCodes.Canceled:
		return false
	case grpcCodes.DeadlineExceeded:
		return false
	}

	return cm.connFactory.ShouldDisposeOfConn(rootErr)
}

func (cm *ConnMgr[T]) Close() {
	cm.cancel()
	cm.closeWG.Wait()
}

func (cm *ConnMgr[T]) run() {
	defer func() {
		cm.connPoolMu.Lock()
		for i, conn := range cm.connPool {
			_ = conn.inner.Close()
			cm.connPool[i] = nil
			cm.idPool.Release(conn.id)
			MetricGRPCConnections.WithLabelValues(cm.connFactory.ServiceName(), conn.Target()).Dec()
		}
		cm.connPool = cm.connPool[:0]
		if cm.connectedCh != nil {
			close(cm.connectedCh)
			cm.connectedCh = nil
		}
		cm.connPoolMu.Unlock()
		cm.closeWG.Done()
	}()
	var nilOrReconnectCh <-chan clock.Time
	for {
		select {
		case <-nilOrReconnectCh:
		case <-cm.reconnectCh:
			logrus.Info("Force connection pool refresh")
		case <-cm.ctx.Done():
			return
		}
		reconnectPeriod, err := cm.refreshConnPool()
		if err != nil {
			// If the client is closing, then return immediately.
			if errors.Is(err, context.Canceled) {
				return
			}
			logrus.WithError(err).Errorf("Failed to refresh connection pool")
			reconnectPeriod = cm.cfg.BackOffTimeout
		}
		// If a server returns zero TTL it means that periodic server list
		// refreshes should be disabled.
		if reconnectPeriod > 0 {
			nilOrReconnectCh = clock.After(reconnectPeriod)
		}
	}
}

func (cm *ConnMgr[T]) refreshConnPool() (clock.Duration, error) {
	begin := clock.Now()
	ctx, cancel := context.WithTimeout(cm.ctx, cm.cfg.RPCTimeout)
	defer cancel()

	getGRPCEndpointRs, err := cm.getServerEndpoints(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "while getting gRPC endpoints")
	}

	// Adjust TTL to be a reasonable value. Zero disables periodic refreshes.
	ttl := clock.Duration(getGRPCEndpointRs.TTL) * clock.Second
	if ttl <= 0 {
		ttl = 0
	} else if ttl < minConnectionTTL {
		ttl = minConnectionTTL
	}

	newConnCount := 0
	crossZoneCount := 0
	logrus.Infof("Connecting to %d %s servers", len(getGRPCEndpointRs.Servers), cm.connFactory.ServiceName())
	for _, serverSpec := range getGRPCEndpointRs.Servers {
		if serverSpec.Zone != cm.cfg.Zone {
			crossZoneCount++
		}
		// Do we have the correct number of connections for this serverSpec in the pool?
		activeConnections := cm.countConnections(serverSpec.Endpoint)
		if activeConnections >= cm.cfg.NumConnections {
			continue
		}

		for i := 0; i < (cm.cfg.NumConnections - activeConnections); i++ {
			conn, err := cm.newConnection(serverSpec.Endpoint)
			if err != nil {
				// If the client is closing, then return immediately.
				if errors.Is(err, context.Canceled) {
					return 0, err
				}
				logrus.WithError(err).Errorf("Failed to dial %s server: %s",
					cm.connFactory.ServiceName(), serverSpec.Endpoint)
				break
			}

			// Add the connection to the pool and notify
			// goroutines waiting for a connection.
			cm.connPoolMu.Lock()
			cm.connPool = append(cm.connPool, conn)
			if cm.connectedCh != nil {
				close(cm.connectedCh)
				cm.connectedCh = nil
			}
			MetricGRPCConnections.WithLabelValues(cm.connFactory.ServiceName(), conn.Target()).Inc()
			cm.connPoolMu.Unlock()
			newConnCount++
			logrus.Infof("Connected to %s server: %s, zone=%s", cm.connFactory.ServiceName(), serverSpec.Endpoint, serverSpec.Zone)
		}
	}
	cm.connPoolMu.Lock()
	connPoolSize := len(cm.connPool)
	// If there has been no new connection established but the pool is not
	// empty then trigger the requested connected notification anyway.
	if connPoolSize > 0 && cm.connectedCh != nil {
		close(cm.connectedCh)
		cm.connectedCh = nil
	}
	cm.connPoolMu.Unlock()
	took := clock.Since(begin).Truncate(clock.Millisecond)
	logrus.Warnf("Connection pool refreshed: took=%s, zone=%s, poolSize=%d, newConnCount=%d, knownServerCount=%d, crossZoneCount=%d, ttl=%s",
		took, cm.cfg.Zone, connPoolSize, newConnCount, len(getGRPCEndpointRs.Servers), crossZoneCount, ttl)
	if connPoolSize < 1 {
		return 0, errConnPoolEmpty
	}
	return ttl, nil
}

// countConnections returns the total number of connections in the pool for the provided endpoint.
func (cm *ConnMgr[T]) countConnections(endpoint string) (count int) {
	cm.connPoolMu.RLock()
	defer cm.connPoolMu.RUnlock()
	for i := 0; i < len(cm.connPool); i++ {
		if cm.connPool[i].Target() == endpoint {
			count++
		}
	}
	return count
}

// newConnection establishes a new GRPC connection to the provided endpoint
func (cm *ConnMgr[T]) newConnection(endpoint string) (*Conn[T], error) {
	// Establish a connection with the server.
	ctx, cancel := context.WithTimeout(cm.ctx, cm.cfg.RPCTimeout)
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
	}
	grpcConn, err := grpc.DialContext(ctx, endpoint, opts...)
	cancel()
	if err != nil {
		return nil, err
	}
	id := cm.idPool.Allocate()
	return &Conn[T]{
		inner:  grpcConn,
		client: cm.connFactory.NewGRPCClient(grpcConn),
		id:     id,
	}, nil
}

func (cm *ConnMgr[T]) getServerEndpoints(ctx context.Context) (*GetGRPCEndpointsRs, error) {
	rq, err := http.NewRequestWithContext(ctx, "GET", cm.getEndpointsURL, http.NoBody)
	if err != nil {
		return nil, errors.Wrap(err, "during request")
	}

	// Override the host header if provided in config
	if cm.cfg.OverrideHostHeader != "" {
		rq.Host = cm.cfg.OverrideHostHeader
	}

	rs, err := cm.httpClt.Do(rq)
	if err != nil {
		return nil, errors.Stack(err)
	}
	defer rs.Body.Close()

	if rs.StatusCode != http.StatusOK {
		return nil, errFromResponse(rs)
	}
	var rsBody GetGRPCEndpointsRs
	if err := readResponseBody(rs, &rsBody); err != nil {
		return nil, errors.Wrap(err, "while unmarshalling response")
	}
	return &rsBody, nil
}

type GenericResponse struct {
	Msg string `json:"message"`
}

func errFromResponse(rs *http.Response) error {
	body, err := io.ReadAll(rs.Body)
	if err != nil {
		return fmt.Errorf("HTTP request error, status=%s", rs.Status)
	}
	defer rs.Body.Close()
	var rsBody GenericResponse
	if err := json.Unmarshal(body, &rsBody); err != nil {
		return errors.Wrapf(err, "HTTP request error, status=%s, body=%s", rs.Status, body)
	}
	return fmt.Errorf("HTTP request error, status=%s, message=%s", rs.Status, rsBody.Msg)
}

// TransCountInTests returns the total number of pending read/write operations.
// It is only supposed to be used in tests, hence it is not exposed in Client
// interface.
func (cm *ConnMgr[T]) TransCountInTests() int {
	transCount := 0
	cm.connPoolMu.RLock()
	for _, serverConn := range cm.connPool {
		transCount += serverConn.counter
	}
	cm.connPoolMu.RUnlock()
	return transCount
}

type GetGRPCEndpointsRs struct {
	Servers []ServerSpec `json:"servers"`
	TTL     int          `json:"ttl"`
	// FIXME: Remove the following fields once all clients are upgraded.
	Endpoint string `json:"grpc_endpoint"`
	Zone     string `json:"zone"`
}

type ServerSpec struct {
	Endpoint  string     `json:"endpoint"`
	Zone      string     `json:"zone"`
	Timestamp clock.Time `json:"timestamp"`
}

func readResponseBody(rs *http.Response, body interface{}) error {
	bodyBytes, err := io.ReadAll(rs.Body)
	if err != nil {
		return errors.Wrap(err, "while reading response")
	}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return errors.Wrapf(err, "while parsing response %s", bodyBytes)
	}
	return nil
}
