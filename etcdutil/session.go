package etcdutil

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/mailgun/holster/v4/setter"
	"github.com/mailgun/holster/v4/syncutil"
	"github.com/pkg/errors"
	etcd "go.etcd.io/etcd/client/v3"
)

const NoLease = etcd.LeaseID(-1)

type SessionObserver func(etcd.LeaseID, error)

type Session struct {
	keepAlive     <-chan *etcd.LeaseKeepAliveResponse
	lease         *etcd.LeaseGrantResponse
	backOff       *backOffCounter
	wg            syncutil.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	observer      SessionObserver
	client        *etcd.Client
	ttl           time.Duration
	lastKeepAlive time.Time
	isRunning     int32
}

type SessionConfig struct {
	TTL      int64
	Observer SessionObserver
}

// NewSession creates a lease and monitors lease keep alive's for connectivity.
// Once a lease ID is granted SessionConfig.Observer is called with the granted lease.
// If connectivity is lost with etcd SessionConfig.Observer is called again with -1 (NoLease)
// as the lease ID. The Session will continue to try to gain another lease, once a new lease
// is gained SessionConfig.Observer is called again with the new lease id.
func NewSession(c *etcd.Client, conf SessionConfig) (*Session, error) {
	setter.SetDefault(&conf.TTL, int64(30))

	if conf.Observer == nil {
		return nil, errors.New("provided observer function cannot be nil")
	}

	if c == nil {
		return nil, errors.New("provided etcd client cannot be nil")
	}

	ttlDuration := time.Second * time.Duration(conf.TTL)
	s := Session{
		observer: conf.Observer,
		ttl:      ttlDuration,
		backOff:  newBackOffCounter(time.Millisecond*500, ttlDuration, 2),
		client:   c,
	}

	s.start()
	return &s, nil
}

func (s *Session) start() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	ticker := time.NewTicker(s.ttl)
	s.lastKeepAlive = time.Now()
	atomic.StoreInt32(&s.isRunning, 1)

	s.wg.Until(func(done chan struct{}) bool {
		// If we have lost our keep alive, attempt to regain it
		if s.keepAlive == nil {
			if err := s.gainLease(s.ctx); err != nil {
				s.observer(NoLease, errors.Wrap(err, "while attempting to gain new lease"))
				select {
				case <-time.After(s.backOff.Next()):
					return true
				case <-s.ctx.Done():
					atomic.StoreInt32(&s.isRunning, 0)
					return false
				}
				// TODO: Fix this in the library. Unreachable code
				// return true
			}
		}
		s.backOff.Reset()

		select {
		case _, ok := <-s.keepAlive:
			if !ok {
				//log.Warn("heartbeat lost")
				s.keepAlive = nil
			} else {
				//log.Debug("heartbeat received")
				s.lastKeepAlive = time.Now()
			}
		case <-ticker.C:
			// Ensure we are getting heartbeats regularly
			if time.Now().Sub(s.lastKeepAlive) > s.ttl {
				//log.Warn("too long between heartbeats")
				s.keepAlive = nil
			}
		case <-done:
			s.keepAlive = nil
			if s.lease != nil {
				ctx, cancel := context.WithTimeout(context.Background(), s.ttl)
				if _, err := s.client.Revoke(ctx, s.lease.ID); err != nil {
					s.observer(NoLease, errors.Wrap(err, "while revoking our lease during shutdown"))
				}
				cancel()
			}
			atomic.StoreInt32(&s.isRunning, 0)
			return false
		}

		if s.keepAlive == nil {
			s.observer(NoLease, nil)
		}
		return true
	})
}

func (s *Session) Reset() {
	if atomic.LoadInt32(&s.isRunning) != 1 {
		return
	}
	s.Close()
	s.start()
}

// Close terminates the session shutting down all network operations,
// then SessionConfig.Observer is called with -1 (NoLease), only returns
// once the session has closed successfully.
func (s *Session) Close() {
	if atomic.LoadInt32(&s.isRunning) != 1 {
		return
	}

	s.cancel()
	s.wg.Stop()
	s.observer(NoLease, nil)
}

func (s *Session) gainLease(ctx context.Context) error {
	var err error
	s.lease, err = s.client.Grant(ctx, int64(s.ttl/time.Second))
	if err != nil {
		return errors.Wrapf(err, "during grant lease")
	}

	s.keepAlive, err = s.client.KeepAlive(s.ctx, s.lease.ID)
	if err != nil {
		return err
	}
	s.observer(s.lease.ID, nil)
	return nil
}
