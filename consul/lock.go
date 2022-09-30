package consul

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/mailgun/holster/v4/errors"
	"github.com/mailgun/holster/v4/setter"
	"github.com/mailgun/holster/v4/syncutil"
	"github.com/sirupsen/logrus"
)

type lock struct {
	wg     syncutil.WaitGroup
	cfg    *LockConfig
	mutex  sync.Mutex
	locked bool
}

type LockConfig struct {
	Client      *api.Client
	LockOptions *api.LockOptions
	Log         *logrus.Entry
	OnChange    func(bool)
}

// Lock attempts to get a lock then continues to keep the lock until told to stop
type Lock interface {
	PutValue(ctx context.Context, b []byte) error
	Unlock(b []byte)
	HasLock() bool
}

// SpawnLock spawns a goroutine to handle lock life cycle. Blocks until the lock is acquired,
// or the context is cancelled. Returns a lock that holds the current state of the lock
func SpawnLock(ctx context.Context, cfg *LockConfig) (*lock, error) {
	l := &lock{
		cfg: cfg,
	}

	setter.SetDefault(&l.cfg.Log, logrus.WithField("category", "consul-lock"))

	// Start acquire lock loop
	errCh := l.spawn(cfg.Client, cfg.LockOptions)

	select {
	case <-errCh:
		return l, nil
	case <-ctx.Done():
		return nil, errors.Wrapf(ctx.Err(), "while waiting for initial lock on '%s'", cfg.LockOptions.Key)
	}
}

func (l *lock) spawn(c *api.Client, opts *api.LockOptions) chan error {
	l.cfg.Log = l.cfg.Log.WithField("lock-name", opts.Key)
	errorCh := make(chan error)

	l.wg.Until(func(done chan struct{}) bool {
		running := true

		// Case where we are looping trying to acquire the
		// lock again but are asked to shutdown
		select {
		case <-done:
			return false
		default:
		}

		// Will only error on invalid config
		lock, err := c.LockOpts(opts)
		if err != nil {
			errorCh <- errors.Wrap(err, "while creating lock")
			return false
		}

		l.cfg.Log.Debug("acquiring lock")
		lockCh, err := lock.Lock(nil)
		if lockCh == nil {
			if err == nil {
				l.cfg.Log.Warn("timeout during lock acquisition; retrying")
				goto RETRY
			}
			l.cfg.Log.WithError(err).Warn("lock acquisition failed; retrying")
			time.Sleep(time.Second)
			goto RETRY
		}

		select {
		case <-lockCh:
			l.cfg.Log.Warn("failed Lock acquisition; another instance trying to claim the lock?; retrying")
			time.Sleep(time.Second)
			goto RETRY
		default:
		}

		l.setLocked(true)
		// We have lock, notify if someone is listening
		select {
		case errorCh <- nil:
		default:
		}

		// Wait for lock to be lost
		select {
		case <-lockCh:
			l.cfg.Log.Warn("lock lost; retrying")
			// Log lock was lost
		case <-done:
			running = false
		}

	RETRY:
		// Release ownership of the lock and cancel the session
		l.cfg.Log.Debug("releasing lock")
		if err := lock.Unlock(); err != nil {
			l.cfg.Log.WithError(err).Warn("while unlocking")
		}
		l.setLocked(false)

		// If we are in shutdown
		if !running {
			if l.cfg.LockOptions.SessionOpts != nil &&
				l.cfg.LockOptions.SessionOpts.Behavior == api.SessionBehaviorDelete {
				if err := lock.Destroy(); err != nil {
					l.cfg.Log.WithError(err).Warn("during lock destroy")
				}
			}
		}
		return running
	})
	return errorCh
}

func (l *lock) setLocked(s bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.cfg.Log.Debugf("Set Lock %t", s)
	if l.cfg.OnChange != nil {
		if l.locked != s {
			l.cfg.OnChange(s)
		}
	}
	l.locked = s
}

func (l *lock) HasLock() bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return l.locked
}

// PutValue stores the given byte slice into the value of the locked key in consul
// returns error if the put failed, also updates the value that will be saved
// when `Unlock()` is called.
func (l *lock) PutValue(ctx context.Context, b []byte) error {
	l.cfg.LockOptions.Value = b
	_, err := l.cfg.Client.KV().Put(&api.KVPair{
		Key:   l.cfg.LockOptions.Key,
		Value: b,
	}, new(api.WriteOptions).WithContext(ctx))
	if err != nil {
		return errors.Wrap(err, "during put for release")
	}
	return nil
}

// Unlock cancels the lock and closes any running goroutines.
func (l *lock) Unlock(b []byte) {
	l.cfg.Log.Infof("Unlock(%s)\n", string(b))
	if b != nil {
		l.cfg.LockOptions.Value = b
	}
	l.wg.Stop()
}

type Mock struct{}

func (*Mock) PutValue(ctx context.Context, b []byte) error { return nil }
func (*Mock) Unlock(b []byte)                              {}
func (*Mock) HasLock() bool                                { return true }
