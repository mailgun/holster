package consul

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/mailgun/holster/v5/errors"
	"github.com/mailgun/holster/v5/setter"
)

// EnvHasConsulConfig returns true if there are items in the local environment
// that have the prefix `CONSUL_`
func EnvHasConsulConfig() bool {
	for _, i := range os.Environ() {
		if strings.HasPrefix(i, "CONSUL_") {
			return true
		}
	}
	return false
}

// NewClient creates a new consul api.Client with the specified config, call
// NewConfig to complete the configuration by reading the local environment
// `CONSUL_` variables.
//
// If no environment variables are set, and the passed cfg is nil, returns
// a client connected to 127.0.0.1:8500
func NewClient(cfg *api.Config) (*api.Client, error) {
	var err error
	if cfg, err = NewConfig(cfg); err != nil {
		return nil, errors.Wrap(err, "failed to build consul config")
	}

	etcdClt, err := api.NewClient(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create consul client")
	}
	return etcdClt, nil
}

// NewConfig creates a new api.Config using environment variables. If an
// existing config is passed, it will fill in missing configuration using
// environment variables or defaults if they exists on the local system.
//
// The config mirrors the same environment variables used by the consul CLI
// as documented here https://www.consul.io/commands
//
// If no environment variables are set, and the passed cfg is nil, returns
// a default config pointing to 127.0.0.1:8500
func NewConfig(cfg *api.Config) (*api.Config, error) {
	setter.SetDefault(&cfg, api.DefaultConfig())

	auth := os.Getenv("CONSUL_HTTP_AUTH")
	if auth != "" {
		parts := strings.Split(auth, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format for 'CONSUL_HTTP_AUTH'; "+
				"expected 'user:pass' got '%s'", auth)
		}
		cfg.HttpAuth = &api.HttpBasicAuth{
			Username: parts[0],
			Password: parts[1],
		}
	}

	setter.SetDefault(&cfg.Address, os.Getenv("CONSUL_HTTP_ADDR"))
	setter.SetDefault(&cfg.Datacenter, os.Getenv("CONSUL_DATACENTER"))
	setter.SetDefault(&cfg.Token, os.Getenv("CONSUL_HTTP_TOKEN"))
	setter.SetDefault(&cfg.TokenFile, os.Getenv("CONSUL_HTTP_TOKEN_FILE"))
	setter.SetDefault(&cfg.Namespace, os.Getenv("CONSUL_NAMESPACE"))
	setter.SetDefault(&cfg.TLSConfig.CertFile, os.Getenv("CONSUL_CLIENT_CERT"))
	setter.SetDefault(&cfg.TLSConfig.KeyFile, os.Getenv("CONSUL_CLIENT_KEY"))
	setter.SetDefault(&cfg.TLSConfig.CAFile, os.Getenv("CONSUL_CACERT"))
	setter.SetDefault(&cfg.TLSConfig.InsecureSkipVerify, getEnvBool("CONSUL_HTTP_SSL_VERIFY"))
	return cfg, nil
}

func getEnvBool(name string) bool {
	v := os.Getenv(name)
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}
