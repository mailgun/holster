package etcdutil

import (
	"context"
	"io/ioutil"
	"sync"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/mailgun/holster"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const NoLease = etcd.LeaseID(-1)

type SessionObserver func(id etcd.LeaseID)

type Session struct {
	keepAlive     <-chan *etcd.LeaseKeepAliveResponse
	lease         *etcd.LeaseGrantResponse
	lastKeepAlive time.Time
	wg            holster.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	conf          SessionConfig
	client        *etcd.Client
	timeout       time.Duration
	once          sync.Once
}

type SessionConfig struct {
	Log      *logrus.Entry
	TTL      int64
	Observer SessionObserver
}

// NulObserver is used if no session observer is provided in the SessionConfig
func NullObserver(s SessionObserver) {}

// NewSession creates a lease and monitors lease keep alive's for connectivity.
// Once a lease ID is granted SessionConfig.Observer is called with the granted lease.
// If connectivity is lost with etcd SessionConfig.Observer is called again with -1 (NoLease)
// as the lease ID. The Session will continue to try to gain another lease, once a new lease
// is gained SessionConfig.Observer is called again with the new lease id.
func NewSession(ctx context.Context, c *etcd.Client, conf SessionConfig) (*Session, error) {
	null := logrus.New()
	null.SetOutput(ioutil.Discard)

	holster.SetDefault(&conf.Log, null.WithField("category", "null"))
	holster.SetDefault(&conf.TTL, int64(30))
	holster.SetDefault(&conf.Observer, NullObserver)

	conf.Log.Debug("New Session")

	if c == nil {
		return nil, errors.New("provided etcd client cannot be nil")
	}

	s := Session{
		timeout: time.Second * time.Duration(conf.TTL),
		conf:    conf,
		client:  c,
	}

	return &s, s.start(ctx)
}

func (s *Session) start(ctx context.Context) (err error) {

	if err = s.gainLease(ctx); err != nil {
		return errors.Wrap(err, "during initial lease grant")
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	ticker := time.NewTicker(s.timeout)
	s.lastKeepAlive = time.Now()

	s.wg.Until(func(done chan struct{}) bool {
		s.conf.Log.Debug("running")
		// If we have lost our keep alive, attempt to regain it
		if s.keepAlive == nil {
			if err = s.gainLease(s.ctx); err != nil {
				s.conf.Log.WithError(err).Error("while attempting to gain new lease")
				select {
				case <-time.After(s.timeout):
					return true
				case <-s.ctx.Done():
					return false
				}
			}
		}

		select {
		case _, ok := <-s.keepAlive:
			if !ok {
				s.conf.Log.Warn("heartbeat lost")
				s.keepAlive = nil
			}
			s.conf.Log.Debug("heartbeat received")
			s.lastKeepAlive = time.Now()

		case <-ticker.C:
			s.conf.Log.Debugf("ticker '%v'", time.Now().Sub(s.lastKeepAlive))
			// Ensure we are getting heartbeats regularly
			if time.Now().Sub(s.lastKeepAlive) > s.timeout {
				s.conf.Log.Warn("too long between heartbeats")
				s.keepAlive = nil
			}
		case <-done:
			s.conf.Log.Debug("ticker")
			if s.lease != nil {
				ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
				if _, err := s.client.Revoke(ctx, s.lease.ID); err != nil {
					s.conf.Log.WithError(err).Error("while revoking our lease during shutdown")
				}
				cancel()
			}
			return false
		}

		if s.keepAlive == nil {
			s.conf.Observer(NoLease)
		}
		return true
	})

	return err
}

// Close terminates the session shutting down all network operations,
// then SessionConfig.Observer is called with -1 (NoLease)
func (s *Session) Close() {
	s.once.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.wg.Stop()
		s.conf.Observer(NoLease)
	})
}

func (s *Session) gainLease(ctx context.Context) error {
	s.conf.Log.Debug("attempting to grant new lease")
	lease, err := s.client.Grant(ctx, s.conf.TTL)
	if err != nil {
		return errors.Wrapf(err, "during grant lease")
	}

	s.keepAlive, err = s.client.KeepAlive(ctx, lease.ID)
	if err != nil {
		return err
	}
	s.conf.Log.Debugf("new lease %d", lease.ID)
	s.conf.Observer(lease.ID)
	return nil
}
