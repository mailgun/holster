package discovery

import (
	"bufio"
	"context"
	"io"
	"net"
	"runtime"
	"sort"
	"strconv"
	"sync"

	ml "github.com/hashicorp/memberlist"
	"github.com/mailgun/holster/v3/clock"
	"github.com/mailgun/holster/v3/errors"
	"github.com/mailgun/holster/v3/retry"
	"github.com/mailgun/holster/v3/setter"
	"github.com/sirupsen/logrus"
)

type Peer struct {
	// An ID the uniquely identifies this peer
	ID string
	// The metadata associated with this peer
	Metadata []byte
	// Is true if this Peer refers to our instance
	IsSelf bool
}

type OnUpdateFunc func([]Peer)

type Members interface {
	// Returns the peers currently registered
	GetPeers(context.Context) ([]Peer, error)
	// Removes our peer from the member list and closes all connections
	Close(context.Context) error
	// TODO: Updates the Peer metadata shared with peers
	//UpdatePeer(context.Context, Peer) error
}

type memberList struct {
	log        logrus.FieldLogger
	memberList *ml.Memberlist
	conf       MemberListConfig
	events     *eventDelegate
}

type MemberListConfig struct {
	// This is the address:port the member list protocol listen for other peers on.
	BindAddress string

	// This is the address:port the member list protocol will advertise to other peers. (Defaults to BindAddress)
	AdvertiseAddress string

	// Metadata about this peer which should be shared with other peers
	Peer Peer

	// A list of peers this member list instance can contact to find other peers.
	KnownPeers []string

	// A callback function which is called when the member list changes.
	OnUpdate OnUpdateFunc

	// An interface through which logging will occur; usually *logrus.Entry
	Logger logrus.FieldLogger
}

func NewMemberList(ctx context.Context, conf MemberListConfig) (Members, error) {
	setter.SetDefault(&conf.Logger, logrus.WithField("category", "member-list"))
	setter.SetDefault(&conf.AdvertiseAddress, conf.BindAddress)
	if conf.Peer.ID == "" {
		return nil, errors.New("Peer.ID cannot be empty")
	}
	if conf.BindAddress == "" {
		return nil, errors.New("BindAddress cannot be empty")
	}

	conf.Peer.IsSelf = false

	m := &memberList{
		log:  conf.Logger,
		conf: conf,
		events: &eventDelegate{
			peers: make(map[string]Peer, 1),
			conf:  conf,
			log:   conf.Logger,
		},
	}

	// Create the member list config
	config, err := m.newMLConfig(conf)
	if err != nil {
		return nil, err
	}

	// Create a new member list instance
	m.memberList, err = ml.Create(config)
	if err != nil {
		return nil, err
	}

	// Attempt to join the member list using a list of known nodes
	err = retry.Until(ctx, retry.Interval(clock.Millisecond*300), func(ctx context.Context, i int) error {
		_, err = m.memberList.Join(m.conf.KnownPeers)
		if err != nil {
			return errors.Wrapf(err, "while joining member list known peers %#v", m.conf.KnownPeers)
		}
		return nil
	})
	return m, errors.Wrap(err, "timed out attempting to join member list")
}

func (m *memberList) newMLConfig(conf MemberListConfig) (*ml.Config, error) {
	config := ml.DefaultWANConfig()
	config.Name = conf.Peer.ID
	config.LogOutput = newLogWriter(conf.Logger)

	var err error
	config.BindAddr, config.BindPort, err = splitAddress(conf.BindAddress)
	if err != nil {
		return nil, errors.Wrap(err, "BindAddress=`%s` is invalid;")
	}

	config.AdvertiseAddr, config.AdvertisePort, err = splitAddress(conf.AdvertiseAddress)
	if err != nil {
		return nil, errors.Wrap(err, "LivelinessAddress=`%s` is invalid;")
	}

	config.Delegate = &delegate{meta: conf.Peer.Metadata}
	config.Events = m.events
	return config, nil
}

func (m *memberList) Close(ctx context.Context) error {
	errCh := make(chan error)
	go func() {
		if err := m.memberList.Leave(clock.Second * 30); err != nil {
			errCh <- err
			return
		}
		errCh <- m.memberList.Shutdown()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (m *memberList) GetPeers(_ context.Context) ([]Peer, error) {
	return m.events.GetPeers()
}

type eventDelegate struct {
	peers map[string]Peer
	log   logrus.FieldLogger
	conf  MemberListConfig
	mutex sync.Mutex
}

func (e *eventDelegate) NotifyJoin(node *ml.Node) {
	defer e.mutex.Unlock()
	e.mutex.Lock()
	e.peers[node.Name] = Peer{ID: node.Name, Metadata: node.Meta}
	e.callOnUpdate()
}

func (e *eventDelegate) NotifyLeave(node *ml.Node) {
	defer e.mutex.Unlock()
	e.mutex.Lock()
	delete(e.peers, node.Name)
	e.callOnUpdate()
}

func (e *eventDelegate) NotifyUpdate(node *ml.Node) {
	defer e.mutex.Unlock()
	e.mutex.Lock()
	e.peers[node.Name] = Peer{ID: node.Name, Metadata: node.Meta}
	e.callOnUpdate()
}
func (e *eventDelegate) GetPeers() ([]Peer, error) {
	defer e.mutex.Unlock()
	e.mutex.Lock()
	return e.getPeers(), nil
}

func (e *eventDelegate) getPeers() []Peer {
	var peers []Peer
	for _, p := range e.peers {
		if p.ID == e.conf.Peer.ID {
			p.IsSelf = true
		}
		peers = append(peers, p)
	}
	return peers
}

func (e *eventDelegate) callOnUpdate() {
	if e.conf.OnUpdate == nil {
		return
	}

	// Sort the results to make it easy to compare peer lists
	peers := e.getPeers()
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].ID < peers[j].ID
	})

	e.conf.OnUpdate(peers)
}

type delegate struct {
	meta []byte
}

func (m *delegate) NodeMeta(int) []byte {
	return m.meta
}
func (m *delegate) NotifyMsg([]byte)                {}
func (m *delegate) GetBroadcasts(int, int) [][]byte { return nil }
func (m *delegate) LocalState(bool) []byte          { return nil }
func (m *delegate) MergeRemoteState([]byte, bool)   {}

func newLogWriter(log logrus.FieldLogger) *io.PipeWriter {
	reader, writer := io.Pipe()

	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			log.Info(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Errorf("Error while reading from Writer: %s", err)
		}
		reader.Close()
	}()
	runtime.SetFinalizer(writer, func(w *io.PipeWriter) {
		writer.Close()
	})

	return writer
}

func split(addr string) (string, int, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return host, 0, errors.New(" expected format is `address:port`")
	}

	intPort, err := strconv.Atoi(port)
	if err != nil {
		return host, intPort, errors.Wrap(err, "port must be a number")
	}
	return host, intPort, nil
}

func splitAddress(addr string) (string, int, error) {
	host, port, err := split(addr)
	if err != nil {
		return "", 0, err
	}
	// Member list requires the address to be an ip address
	if ip := net.ParseIP(host); ip == nil {
		addresses, err := net.LookupHost(host)
		if err != nil {
			return "", 0, errors.Wrapf(err, "while preforming host lookup for '%s'", host)
		}
		if len(addresses) == 0 {
			return "", 0, errors.Wrapf(err, "net.LookupHost() returned no addresses for '%s'", host)
		}
		host = addresses[0]
	}
	return host, port, nil
}
