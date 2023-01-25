package discovery

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
	"github.com/mailgun/holster/v5/cancel"
	"github.com/mailgun/holster/v5/consul"
	"github.com/mailgun/holster/v5/errors"
	"github.com/mailgun/holster/v5/setter"
	"github.com/mailgun/holster/v5/syncutil"
	"github.com/sirupsen/logrus"
)

type ConsulConfig struct {
	// This is the consul client config; typically created by calling api.DefaultConfig()
	ClientConfig *api.Config

	// The name of the catalog we should register under; should be common to all peers in the catalog
	CatalogName string

	// Information about this peer which should be shared with all other peers in the catalog
	Peer Peer

	// This is an address the will be registered with consul so it can preform liveliness checks
	LivelinessAddress string

	// A callback function which is called when the member list changes
	OnUpdate OnUpdateFunc

	// An interface through which logging will occur; usually *logrus.Entry
	Logger logrus.FieldLogger
}

type Consul struct {
	wg     syncutil.WaitGroup
	log    logrus.FieldLogger
	client *api.Client
	plan   *watch.Plan
	conf   *ConsulConfig
	ctx    cancel.Context
}

func NewConsul(conf *ConsulConfig) (Members, error) {
	setter.SetDefault(&conf.Logger, logrus.WithField("category", "consul-catalog"))
	setter.SetDefault(&conf.ClientConfig, api.DefaultConfig())
	var err error

	if conf.Peer.ID == "" {
		return nil, errors.New("Peer.ID cannot be empty")
	}

	if conf.CatalogName == "" {
		return nil, errors.New("CatalogName cannot be empty")
	}

	cs := Consul{
		ctx:  cancel.New(context.Background()),
		log:  conf.Logger,
		conf: conf,
	}

	cs.client, err = api.NewClient(cs.conf.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "while creating a new client")
	}

	// Register ourselves in consul as a member of the cluster
	err = cs.client.Agent().ServiceRegisterOpts(&api.AgentServiceRegistration{
		Name:    conf.CatalogName,
		ID:      conf.Peer.ID,
		Tags:    []string{"scout-bloom"},
		Address: conf.LivelinessAddress,
		Check: &api.AgentServiceCheck{
			DeregisterCriticalServiceAfter: "10m",
			TTL:                            "10s",
		},
		Meta: map[string]string{
			"peer": string(conf.Peer.Metadata),
		},
	}, api.ServiceRegisterOpts{ReplaceExistingChecks: true})
	if err != nil {
		return nil, errors.Wrapf(err, "while registering the peer '%s' to the service catalog '%s'",
			conf.Peer.ID, cs.conf.CatalogName)
	}

	// Update the service check TTL
	err = cs.client.Agent().UpdateTTL(fmt.Sprintf("service:%s", conf.Peer.ID), "", api.HealthPassing)
	if err != nil {
		return nil, errors.Wrap(err, "while updating service TTL after registration")
	}

	cs.log.Debugf("Registered '%s' with consul catalog '%s'", conf.Peer.ID, conf.CatalogName)

	// Periodically update the TTL check on the registered service
	ticker := time.NewTicker(time.Second * 4)
	cs.wg.Until(func(done chan struct{}) bool {
		select {
		case <-ticker.C:
			err := cs.client.Agent().UpdateTTL(fmt.Sprintf("service:%s", conf.Peer.ID), "", api.HealthPassing)
			if err != nil {
				cs.log.WithError(err).Warn("while updating consul TTL")
			}
		case <-done:
			ticker.Stop()
			return false
		}
		return true
	})

	// Watch for changes to the service list and partition config changes
	if err := cs.watch(); err != nil {
		return nil, err
	}

	return &cs, nil
}

func (cs *Consul) watch() error {
	changeCh := make(chan []*api.ServiceEntry, 100)
	var previousPeers map[string]Peer
	var err error

	cs.plan, err = watch.Parse(map[string]interface{}{
		"type":    "service",
		"service": cs.conf.CatalogName,
	})
	if err != nil {
		return fmt.Errorf("while creating watch plan: %s", err)
	}

	cs.plan.HybridHandler = func(blockParamVal watch.BlockingParamVal, raw interface{}) {
		if raw == nil {
			cs.log.Info("Raw == nil")
		}
		if v, ok := raw.([]*api.ServiceEntry); ok && v != nil {
			changeCh <- v
		}
	}

	allChecksPassing := func(checks api.HealthChecks) bool {
		for _, c := range checks {
			if c.Status != "passing" {
				return false
			}
		}
		return true
	}

	cs.wg.Go(func() {
		if err := cs.plan.RunWithClientAndHclog(cs.client, consul.NewHCLogAdapter(cs.log, "consul-store")); err != nil {
			cs.log.WithError(err).Error("Service watch failed")
		}
	})

	cs.wg.Until(func(done chan struct{}) bool {
		select {
		case <-done:
			return false
		case serviceEntries := <-changeCh:
			if cs.conf.OnUpdate == nil {
				return true
			}
			peers := make(map[string]Peer)
			for _, se := range serviceEntries {
				if !allChecksPassing(se.Checks) {
					break
				}
				meta, ok := se.Service.Meta["peer"]
				if !ok {
					cs.log.Errorf("service entry missing 'peer' metadata '%s'", se.Service.ID)
				}
				p := Peer{ID: se.Service.ID, Metadata: []byte(meta)}
				if meta == string(cs.conf.Peer.Metadata) {
					p.IsSelf = true
				}
				peers[p.ID] = p
			}

			if !reflect.DeepEqual(previousPeers, peers) {
				var result []Peer
				for _, v := range peers {
					result = append(result, v)
				}
				// Sort the results to make it easy to compare peer lists
				sort.Slice(result, func(i, j int) bool {
					return result[i].ID < result[j].ID
				})
				cs.conf.OnUpdate(result)
				previousPeers = peers
			}
		}
		return true
	})
	return nil
}

func (cs *Consul) GetPeers(ctx context.Context) ([]Peer, error) {
	opts := &api.QueryOptions{LocalOnly: true}
	services, _, err := cs.client.Health().Service(cs.conf.CatalogName, "", true, opts.WithContext(ctx))
	if err != nil {
		return nil, errors.Wrap(err, "while fetching healthy catalog listing")
	}
	var peers []Peer
	for _, i := range services {
		v, ok := i.Service.Meta["peer"]
		if !ok {
			return nil, fmt.Errorf("service entry missing 'peer' metadata '%s'", i.Service.ID)
		}
		var p Peer
		p.Metadata = []byte(v)
		p.ID = i.Service.ID
		if v == string(cs.conf.Peer.Metadata) {
			p.IsSelf = true
		}
		peers = append(peers, p)
	}
	return peers, nil
}

func (cs *Consul) Close(ctx context.Context) error {
	errCh := make(chan error)
	go func() {
		cs.plan.Stop()
		cs.wg.Stop()
		errCh <- cs.client.Agent().ServiceDeregister(cs.conf.Peer.ID)
	}()

	select {
	case <-ctx.Done():
		cs.ctx.Cancel()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
