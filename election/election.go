package election

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
	"path"
	"sync/atomic"

	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/mailgun/holster"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/grpclog"
)

const (
	pathToCA            = "/etc/mailgun/ssl/localhost/ca.pem"
	localSecureEndpoint = "https://127.0.0.1:2379"
)

type LeaderElector interface {
	Watch(context.Context, uint64) chan bool
	IsLeader() bool
	Concede() bool
	Start()
	Stop()
}

type EtcdElection struct {
	session  *concurrency.Session
	election *concurrency.Election
	client   *etcd.Client
	cancel   context.CancelFunc
	conf     Config
	wg       holster.WaitGroup
	ctx      context.Context
	isLeader int32
}

type Config struct {
	// The name of the election (IE: scout, blackbird, etc...)
	Election string
	// The name of this instance (IE: worker-n01, worker-n02, etc...)
	Candidate string
	// Use this config if provided
	EtcdConfig *etcd.Config
}

func NewEtcdConfig(conf Config) (*etcd.Config, error) {
	var envEndpoint, envUser, envPass, envDebug,
		tlsCertFile, tlsKeyFile, tlsCaCertFile string

	if conf.EtcdConfig != nil {
		return conf.EtcdConfig, nil
	}

	for _, i := range []struct {
		env string
		dst *string
		def string
	}{
		{"ETCD3_ENDPOINT", &envEndpoint, localSecureEndpoint},
		{"ETCD3_USER", &envUser, ""},
		{"ETCD3_PASSWORD", &envPass, ""},
		{"ETCD3_DEBUG", &envDebug, ""},
		{"ETCD3_TLS_CERT", &tlsCertFile, ""},
		{"ETCD3_TLS_KEY", &tlsKeyFile, ""},
		{"ETCD3_CA", &tlsCaCertFile, pathToCA},
	} {
		holster.SetDefault(i.dst, os.Getenv(i.env), i.def)
	}

	if envDebug != "" {
		grpclog.SetLoggerV2(grpclog.NewLoggerV2WithVerbosity(os.Stderr, os.Stderr, os.Stderr, 4))
	}

	tlsConf := tls.Config{
		InsecureSkipVerify: true,
	}

	// If the CA file exists use that
	if _, err := os.Stat(tlsCaCertFile); err == nil {
		var rpool *x509.CertPool = nil
		if pemBytes, err := ioutil.ReadFile(tlsCaCertFile); err == nil {
			rpool = x509.NewCertPool()
			rpool.AppendCertsFromPEM(pemBytes)
		} else {
			return nil, errors.Errorf("while loading cert CA file '%s': %s", tlsCaCertFile, err)
		}
		tlsConf.RootCAs = rpool
		tlsConf.InsecureSkipVerify = false
	}

	if tlsCertFile != "" && tlsKeyFile != "" {
		tlsCert, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
		if err != nil {
			return nil, errors.Errorf("while loading cert '%s' and key file '%s': %s",
				tlsCertFile, tlsKeyFile, err)
		}
		tlsConf.Certificates = []tls.Certificate{tlsCert}
	}

	return &etcd.Config{
		Endpoints: []string{envEndpoint},
		Username:  envUser,
		Password:  envPass,
		TLS:       &tlsConf,
	}, nil
}

// Use leader election if you have several instances of a service running in production
// and you only want one of the service instances to preform a periodic task.
//
//	election, _ := election.New(election.Config{
//      Election: "election-name",
//	})
//
//  // Start the leader election and attempt to become leader
//  election.Start()
//
//	// Returns true if we are leader (thread safe)
//	if election.IsLeader() {
//		// Do periodic thing
//	}
func New(conf Config) (*EtcdElection, error) {
	etcdConf, err := NewEtcdConfig(conf)
	if err != nil {
		return nil, err
	}

	client, err := etcd.New(*etcdConf)
	if err != nil {
		return nil, err
	}

	return NewFromClient(conf, client)
}

func NewFromClient(conf Config, client *etcd.Client) (*EtcdElection, error) {
	if host, err := os.Hostname(); err == nil {
		holster.SetDefault(&conf.Candidate, host)
	}
	// Set a prefix key for elections
	conf.Election = path.Join("/elections", conf.Election)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	_, err := client.Get(ctx, conf.Election)
	if err != nil {
		return nil, errors.Wrap(err, "while connecting to etcd")
	}

	session, err := concurrency.NewSession(client)
	if err != nil {
		return nil, errors.Wrap(err, "while creating new session")
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	leader := &EtcdElection{
		session: session,
		client:  client,
		cancel:  cancelFunc,
		conf:    conf,
		ctx:     ctx,
	}
	return leader, nil
}

func (s *EtcdElection) Start() error {
	// Start a new election
	s.election = concurrency.NewElection(s.session, s.conf.Election)

	if err := s.election.Campaign(s.ctx, s.conf.Candidate); err != nil {
		errors.Wrap(err, "while starting a new campaign")
	}
	s.wg.Until(func(done chan struct{}) bool {
		observeChan := s.election.Observe(s.ctx)
		for {
			select {
			case node := <-observeChan:
				if string(node.Kvs[0].Value) == s.conf.Candidate {
					logrus.WithField("category", "election").
						Debug("IS Leader")
					atomic.StoreInt32(&s.isLeader, 1)
				}
				// We are not leader
				logrus.WithField("category", "election").
					Debug("NOT Leader")
				atomic.StoreInt32(&s.isLeader, 0)
			case <-done:
				return false
			}
		}
	})
	return nil
}

func (s *EtcdElection) Stop() {
	s.cancel()
	s.wg.Wait()
}

func (s *EtcdElection) IsLeader() bool {
	return atomic.LoadInt32(&s.isLeader) == 1
}

// Release leadership and return true if we own it, else do nothing and return false
func (s *EtcdElection) Concede() bool {
	if atomic.LoadInt32(&s.isLeader) == 1 {
		if err := s.election.Resign(s.ctx); err != nil {
			logrus.WithFields(logrus.Fields{
				"category": "election",
				"err":      err,
			}).Error("while attempting to concede the election")
		}
		atomic.StoreInt32(&s.isLeader, 0)
		return true
	}
	return false
}

type LeaderElectionMock struct{}

func (s *LeaderElectionMock) IsLeader() bool { return true }
func (s *LeaderElectionMock) Concede() bool  { return true }
func (s *LeaderElectionMock) Start()         {}
func (s *LeaderElectionMock) Stop()          {}
