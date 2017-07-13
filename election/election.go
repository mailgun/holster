package election

import (
	"context"
	"os"
	"path"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	etcd "github.com/coreos/etcd/client"
	"github.com/mailgun/holster"
)

type LeaderElection struct {
	cancel   context.CancelFunc
	conf     Config
	wg       holster.WaitGroup
	ctx      context.Context
	api      etcd.KeysAPI
	isLeader int32
	conceded int32
}

type Config struct {
	// The name of the election (IE: scout, blackbird, etc...)
	Election string
	// The name of this instance (IE: worker-n01, worker-n02, etc...)
	Candidate string

	Endpoints []string
	TTL       time.Duration
}

// Use leader election if you have several instances of a service running in production
// and you only want one of the service instances to preform a periodic task.
//
//	leader, _ := holster.NewLeaderElection(holster.LeaderElectionConf{
//		Endpoints:     []string{"http://192.168.99.100:2379"},
//	})
//
//	// Returns true if we are leader (thread safe)
//	if leader.IsLeader() {
//		// Do periodic thing
//	}
func New(conf Config) (*LeaderElection, error) {
	client, err := etcd.New(etcd.Config{
		Endpoints: conf.Endpoints,
	})
	if err != nil {
		return nil, err
	}

	if host, err := os.Hostname(); err != nil {
		holster.SetDefault(&conf.Candidate, host)
	}
	holster.SetDefault(&conf.Election, "default-election")
	holster.SetDefault(&conf.TTL, time.Second*5)

	// Set a root prefix for `/elections`
	conf.Election = path.Join("elections", conf.Election)

	ctx, cancelFunc := context.WithCancel(context.Background())
	go func() {
		for {
			// Keep our client update to date with all etcd nodes
			err := client.AutoSync(ctx, 10*time.Second)
			if err == context.DeadlineExceeded || err == context.Canceled {
				break
			}
			logrus.Infof("LeaderElection sync: %s", err)
			time.Sleep(time.Second * 10)
		}
	}()

	leader := &LeaderElection{
		api:    etcd.NewKeysAPI(client),
		cancel: cancelFunc,
		conf:   conf,
		ctx:    ctx,
	}
	leader.Start()
	return leader, nil
}

// Watch the prefix for any events and send a 'true' if we should attempt grab leader
func (s *LeaderElection) Watch(ctx context.Context, watcher etcd.Watcher) chan bool {
	results := make(chan bool)
	var cancel atomic.Value

	go func() {
		select {
		case <-ctx.Done():
			cancel.Load().(context.CancelFunc)()
			return
		}
	}()

	s.wg.Loop(func() bool {
		ctx, c := context.WithTimeout(context.Background(), time.Second*60)
		cancel.Store(c)
		resp, err := watcher.Next(ctx)
		if err != nil {
			if err == context.Canceled {
				close(results)
				return false
			}
			if err != context.DeadlineExceeded {
				logrus.Errorf("LeaderElection watch: %s", err)
				time.Sleep(time.Second * 10)
			}
		} else {
			logrus.Debug("LeaderElection etcd event: %+v\n", resp)
			results <- true
		}
		return true
	})
	return results
}

func (s *LeaderElection) Start() {
	opts := etcd.SetOptions{
		PrevExist: etcd.PrevNoExist,
		TTL:       s.conf.TTL,
	}
	ticker := time.NewTicker(s.conf.TTL * 3 / 4)
	var index uint64

	// Attempt to become leader
	resp, err := s.api.Set(s.ctx, s.conf.Election, s.conf.Candidate, &opts)
	if err == nil {
		atomic.StoreInt32(&s.isLeader, 1)
		index = resp.Index
	} else {
		if cast, ok := err.(etcd.Error); ok {
			index = cast.Index
		}
	}
	// Watch our election for changes
	event := s.Watch(s.ctx, s.api.Watcher(s.conf.Election, &etcd.WatcherOptions{AfterIndex: index}))

	// Keep trying
	s.wg.Loop(func() bool {
		select {
		case <-ticker.C:
			// If we are leader
			if atomic.LoadInt32(&s.isLeader) == 1 {
				// Refresh our leader status
				opts := etcd.SetOptions{
					Refresh: true,
					TTL:     s.conf.TTL,
				}
				if _, err := s.api.Set(s.ctx, s.conf.Election, "", &opts); err != nil {
					atomic.StoreInt32(&s.isLeader, 0)
				}
				return true
			}
		case <-event:
			// If we recently conceded leadership, ignore this event and wait for the next one
			if atomic.LoadInt32(&s.conceded) == 1 {
				atomic.StoreInt32(&s.conceded, 0)
				return true
			}
			// If we are not leader
			if atomic.LoadInt32(&s.isLeader) == 0 {
				// Attempt to become leader
				if _, err := s.api.Set(s.ctx, s.conf.Election, s.conf.Candidate, &opts); err == nil {
					atomic.StoreInt32(&s.isLeader, 1)
				}
			}
		case <-s.ctx.Done():
			// Give up leadership if we are leader
			if atomic.LoadInt32(&s.isLeader) == 1 {
				s.api.Delete(context.Background(), s.conf.Election, nil)
			}
			atomic.StoreInt32(&s.isLeader, 0)
			ticker.Stop()
			return false
		}
		return true
	})
}

func (s *LeaderElection) Stop() {
	s.cancel()
	s.wg.Wait()
}

func (s *LeaderElection) IsLeader() bool {
	return atomic.LoadInt32(&s.isLeader) == 1
}

// Release leadership and return true if we own it, else do nothing and return false
func (s *LeaderElection) Concede() bool {
	if atomic.LoadInt32(&s.isLeader) == 1 {
		atomic.StoreInt32(&s.conceded, 1)
		s.api.Delete(context.Background(), s.conf.Election, nil)
		atomic.StoreInt32(&s.isLeader, 0)
		return true
	}
	return false
}
