package etcdutil_test

import (
	"context"
	"crypto/tls"
	"os"
	"testing"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/mailgun/holster/etcdutil"
	. "gopkg.in/check.v1"
)

const (
	localInsecureEndpoint = "http://127.0.0.1:23790"
	localSecureEndpoint   = "https://127.0.0.1:23790"
)

func TestConfig(t *testing.T) {
	TestingT(t)
}

type ConfigSuite struct{}

var _ = Suite(&ConfigSuite{})

func (s *ConfigSuite) TearDownTest(c *C) {
	unStashEnv("ETCD3_USER")
	unStashEnv("ETCD3_PASSWORD")
	unStashEnv("ETCD3_ENDPOINT")
}

func (s *ConfigSuite) TestApplyDefault(c *C) {
	stashEnv("ETCD3_USER", "")
	stashEnv("ETCD3_PASSWORD", "")
	stashEnv("ETCD3_ENDPOINT", "")
	stashEnv("ETCD3_TLS_CERT", "")
	stashEnv("ETCD3_TLS_KEY", "")
	stashEnv("ETCD3_CA", "")

	var cfg *etcd.Config
	var err error

	cfg, err = etcdutil.NewEtcdConfig(cfg)
	c.Assert(err, IsNil)
	c.Assert(cfg.Endpoints[0], Equals, localInsecureEndpoint)
	c.Assert(cfg.TLS, IsNil)
}

func (s *ConfigSuite) TestApplyDefaultWithCreds(c *C) {
	stashEnv("ETCD3_TLS_CERT", "tls/server.pem")
	stashEnv("ETCD3_TLS_KEY", "tls/server-key.pem")
	stashEnv("ETCD3_SKIP_VERIFY", "true")
	stashEnv("ETCD3_PASSWORD", "pass")
	stashEnv("ETCD3_USER", "user")
	stashEnv("ETCD3_ENDPOINT", "")
	stashEnv("ETCD3_CA", "tls/ca.pem")

	var cfg *etcd.Config
	var err error

	cfg, err = etcdutil.NewEtcdConfig(cfg)
	c.Assert(err, IsNil)
	c.Assert(cfg.Endpoints[0], Equals, localSecureEndpoint)

	c.Assert(cfg.TLS, NotNil)
	c.Assert(cfg.TLS.InsecureSkipVerify, Equals, true)

	c.Assert(cfg.Username, Equals, "user")
	c.Assert(cfg.Password, Equals, "pass")
}

func (s *ConfigSuite) TestApplyDefaultPreferSetValue(c *C) {
	stashEnv("ETCD3_ENDPOINT", "http://example.com")
	stashEnv("ETCD3_PASSWORD", "pass")
	stashEnv("ETCD3_USER", "user")

	cfg := &etcd.Config{
		Endpoints: []string{"https://foo.bar"},
		TLS: &tls.Config{
			InsecureSkipVerify: false,
		},
		Username: "kit",
		Password: "kat",
	}

	var err error
	cfg, err = etcdutil.NewEtcdConfig(cfg)
	c.Assert(err, IsNil)

	c.Assert(cfg.Endpoints[0], Equals, "https://foo.bar")

	c.Assert(cfg.TLS, NotNil)
	c.Assert(cfg.TLS.InsecureSkipVerify, Equals, false)

	c.Assert(cfg.Username, Equals, "kit")
	c.Assert(cfg.Password, Equals, "kat")
}

func (s *ConfigSuite) TestNewSecureClient(c *C) {
	stashEnv("ETCD3_ENDPOINT", "http://example.com")
	stashEnv("ETCD3_PASSWORD", "pass")
	stashEnv("ETCD3_USER", "user")
	var err error

	// This assumes we used to `docker-compose up` to create the etcd node
	cfg := &etcd.Config{
		Endpoints: []string{"https://localhost:2379"},
		Username:  "root",
		Password:  "rootpw",
		TLS: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client, err := etcd.New(*cfg)
	c.Assert(err, IsNil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	_, err = client.Put(ctx, "/test-thing", "HI1")
	c.Assert(err, IsNil)

	resp, err := client.Get(ctx, "/test-thing")
	c.Assert(err, IsNil)

	c.Assert(string(resp.Kvs[0].Value), Equals, "HI1")

	_, err = client.Delete(ctx, "/test-thing")
	c.Assert(err, IsNil)
}

var stash map[string]string

func stashEnv(name, value string) {
	if stash == nil {
		stash = make(map[string]string)
	}
	stash[name] = os.Getenv(name)
	os.Setenv(name, value)
}

func unStashEnv(name string) {
	if stash == nil {
		return
	}
	value, ok := stash[name]
	if ok {
		os.Setenv(name, value)
	}
}
