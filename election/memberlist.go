package election

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"runtime"
	"strconv"

	ml "github.com/hashicorp/memberlist"
	"github.com/mailgun/holster/v3/clock"
	"github.com/mailgun/holster/v3/errors"
	"github.com/mailgun/holster/v3/retry"
	"github.com/mailgun/holster/v3/setter"
	"github.com/sirupsen/logrus"
)

type PeerInfo struct {
	// The http address:port of the peer
	HTTPAddress string `json:"http-address"`
	// The grpc address:port of the peer
	GRPCAddress string `json:"grpc-address"`
	// Is true if PeerInfo matches the Name as given in the memberlist config
	IsOwner bool `json:"is-owner,omitempty"`
}

type UpdateFunc func([]PeerInfo)

type MemberListPool struct {
	log        logrus.FieldLogger
	memberList *ml.Memberlist
	conf       MemberListPoolConfig
	events     *memberListEventHandler
}

type MemberListPoolConfig struct {
	// This is the address:port the member list protocol listen for other members on
	MemberListAddress string

	// The information about this peer which should be shared with all other members
	PeerInfo PeerInfo

	// A list of nodes this member list instance can contact to find other members.
	KnownNodes []string

	// A callback function which is called when the member list changes
	OnUpdate UpdateFunc

	// An interface through which logging will occur (Usually *logrus.Entry)
	Logger logrus.FieldLogger
}

func NewMemberListPool(ctx context.Context, conf MemberListPoolConfig) (*MemberListPool, error) {
	setter.SetDefault(conf.Logger, logrus.WithField("category", "gubernator"))
	m := &MemberListPool{
		log:  conf.Logger,
		conf: conf,
	}

	host, port, err := splitAddress(conf.MemberListAddress)
	if err != nil {
		return nil, errors.Wrap(err, "MemberListAddress=`%s` is invalid;")
	}

	// Member list requires the address to be an ip address
	if ip := net.ParseIP(host); ip == nil {
		addrs, err := net.LookupHost(host)
		if err != nil {
			return nil, errors.Wrapf(err, "while preforming host lookup for '%s'", host)
		}
		if len(addrs) == 0 {
			return nil, errors.Wrapf(err, "net.LookupHost() returned no addresses for '%s'", host)
		}
		host = addrs[0]
	}

	// Configure member list event handler
	m.events = &memberListEventHandler{
		conf: conf,
		log:  m.log,
	}
	m.events.peers = make(map[string]PeerInfo)

	// Configure member list
	config := ml.DefaultWANConfig()
	config.Events = m.events
	config.AdvertiseAddr = host
	config.AdvertisePort = port
	config.Name = conf.PeerInfo.HTTPAddress
	config.LogOutput = newLogWriter(m.log)

	// Create and set member list
	memberList, err := ml.Create(config)
	if err != nil {
		return nil, err
	}
	m.memberList = memberList
	conf.PeerInfo.IsOwner = false

	// Join member list pool
	err = m.joinPool(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "while attempting to join the member-list pool")
	}

	return m, nil
}

func (m *MemberListPool) joinPool(ctx context.Context) error {
	// Get local node and set metadata
	node := m.memberList.LocalNode()
	b, err := json.Marshal(&m.conf.PeerInfo)
	if err != nil {
		return errors.Wrap(err, "error marshalling metadata")
	}
	node.Meta = b

	err = retry.Until(ctx, retry.Interval(clock.Millisecond*300), func(ctx context.Context, i int) error {
		// Join member list
		_, err = m.memberList.Join(m.conf.KnownNodes)
		if err != nil {
			return errors.Wrap(err, "while joining member-list")
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "timed out attempting to join member list")
	}

	// Add the local node to the event handler's peer list
	m.events.addPeer(node)
	return nil
}

func (m *MemberListPool) Close() {
	err := m.memberList.Leave(clock.Second)
	if err != nil {
		m.log.Warn(errors.Wrap(err, "while leaving member-list"))
	}
}

type memberListEventHandler struct {
	peers map[string]PeerInfo
	log   logrus.FieldLogger
	conf  MemberListPoolConfig
}

func (e *memberListEventHandler) addPeer(node *ml.Node) {
	ip := getIP(node.Address())

	// Deserialize metadata
	var metadata PeerInfo
	if err := json.Unmarshal(node.Meta, &metadata); err != nil {
		e.log.WithError(err).Warnf("while adding to peers")
		return
	}
	e.peers[ip] = metadata
	e.callOnUpdate()
}

func (e *memberListEventHandler) NotifyJoin(node *ml.Node) {
	ip := getIP(node.Address())

	// Deserialize metadata
	var metadata PeerInfo
	if err := json.Unmarshal(node.Meta, &metadata); err != nil {
		e.log.WithError(err).Warn("while deserialize member-list metadata")
		return
	}
	e.peers[ip] = metadata
	e.callOnUpdate()
}

func (e *memberListEventHandler) NotifyLeave(node *ml.Node) {
	ip := getIP(node.Address())

	// Remove PeerInfo
	delete(e.peers, ip)

	e.callOnUpdate()
}

func (e *memberListEventHandler) NotifyUpdate(node *ml.Node) {
	ip := getIP(node.Address())

	// Deserialize metadata
	var metadata PeerInfo
	if err := json.Unmarshal(node.Meta, &metadata); err != nil {
		e.log.WithError(err).Warn("while updating member-list")
		return
	}
	e.peers[ip] = metadata
	e.callOnUpdate()
}

func (e *memberListEventHandler) callOnUpdate() {
	var peers []PeerInfo

	for _, p := range e.peers {
		if p.HTTPAddress == e.conf.PeerInfo.HTTPAddress {
			p.IsOwner = true
		}
		peers = append(peers, p)
	}
	e.conf.OnUpdate(peers)
}

func getIP(address string) string {
	addr, _, _ := net.SplitHostPort(address)
	return addr
}

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

func splitAddress(addr string) (string, int, error) {
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
