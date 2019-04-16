package etcdutil

import (
	"context"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/mailgun/holster"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

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
}

type SessionConfig struct {
	Log      *logrus.Entry
	Timeout  time.Duration
	Observer SessionObserver
}

func NewSession(ctx context.Context, c *etcd.Client, conf SessionConfig) (*Session, error) {
	// TODO: Check for Log
	// TODO: Set default timeout
	// TODO: Provide a dummy observer
	// TODO: Require etcd client
	s := Session{
		client: c,
		conf:   conf,
	}
	return &s, s.start(ctx)
}

// TODO: How do we shutdown (Add Stop())
// TODO: What does public interface look like. (Write a test)
func (s *Session) start(ctx context.Context) (err error) {

	if err = s.gainLease(ctx); err != nil {
		return errors.Wrap(err, "during initial lease grant")
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.wg.Until(func(done chan struct{}) bool {
		// If we have lost our keep alive, attempt to regain it
		if s.keepAlive == nil {
			if err = s.gainLease(s.ctx); err != nil {
				s.conf.Log.WithError(err).Error("while attempting to gain lease")
				select {
				case <-time.After(s.conf.Timeout):
					return true
				case <-s.ctx.Done():
					return false
				}
			}
		}

		select {
		case _, ok := <-s.keepAlive:
			if !ok {
				s.conf.Log.Warn("keep alive lost")
				s.keepAlive = nil
			}

			// Ensure we are getting keep alive's regularly
			if s.lastKeepAlive.Sub(time.Now()) > s.conf.Timeout {
				s.conf.Log.Warn("too long between keep alive heartbeats")
				s.keepAlive = nil
			}
			s.lastKeepAlive = time.Now()
		case <-done:
			if s.lease != nil {
				ctx, cancel := context.WithTimeout(context.Background(), s.conf.Timeout)
				if _, err := s.conf.Client.Revoke(ctx, s.lease.ID); err != nil {
					s.conf.Log.WithError(err).Error("while revoking our lease during shutdown")
				}
				cancel()
			}
			return false
		}

		if s.keepAlive == nil {
			s.conf.Observer(ElectionEvent{
				Event: EventLostLease,
			})
		}
		return true
	})
	return err
}

func (e *Session) gainLease(ctx context.Context) error {
	e.emitLog(nil, "attempting to grant new lease")
	lease, err := e.client.Grant(ctx, e.conf.TTL)
	if err != nil {
		return errors.Wrapf(err, "during grant lease")
	}

	keepAlive, err := e.client.KeepAlive(e.ctx, lease.ID)
	if err != nil {
		return err
	}
	s.keepAlive = keepAlive
	e.emit(ElectionEvent{
		Event:   EventGainLease,
		LeaseID: lease.ID,
	})
	return nil
}
