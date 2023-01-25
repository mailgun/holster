package consul_test

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/mailgun/holster/v5/consul"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientTLS(t *testing.T) {
	t.Skip("TODO: https://github.com/mailgun/holster/issues/151")

	t.Setenv("CONSUL_HTTP_ADDR", "https://127.0.0.1:8501")
	t.Setenv("CONSUL_CLIENT_CERT", "config/dc1-server-consul-0.pem")
	t.Setenv("CONSUL_CLIENT_KEY", "config/dc1-server-consul-0-key.pem")
	t.Setenv("CONSUL_CACERT", "config/consul-agent-ca.pem")

	client, err := consul.NewClient(nil)
	require.NoError(t, err)

	kv := api.KVPair{
		Key:   "test-key-tls",
		Value: []byte("test-value-tls"),
	}
	_, err = client.KV().Put(&kv, nil)
	require.NoError(t, err)
	resp, _, err := client.KV().Get("test-key-tls", nil)
	require.NoError(t, err)
	assert.Equal(t, resp.Key, "test-key-tls")
	assert.Equal(t, resp.Value, []byte("test-value-tls"))
}

func TestNewClient(t *testing.T) {
	client, err := consul.NewClient(nil)
	require.NoError(t, err)

	kv := api.KVPair{
		Key:   "test-key",
		Value: []byte("test-value"),
	}
	_, err = client.KV().Put(&kv, nil)
	require.NoError(t, err)
	resp, _, err := client.KV().Get("test-key", nil)
	require.NoError(t, err)
	assert.Equal(t, resp.Key, "test-key")
	assert.Equal(t, resp.Value, []byte("test-value"))
}

func TestEnvHasConsulConfig(t *testing.T) {
	t.Setenv("CONSUL_HTTP_ADDR", "127.0.0.1:8500")
	assert.True(t, consul.EnvHasConsulConfig())
}

func TestNewConfig(t *testing.T) {
	t.Setenv("CONSUL_HTTP_AUTH", "username:password")
	t.Setenv("CONSUL_HTTP_SSL_VERIFY", "true")

	cfg := api.DefaultConfig()
	cfg, err := consul.NewConfig(cfg)
	require.NoError(t, err)

	assert.Equal(t, "username", cfg.HttpAuth.Username)
	assert.Equal(t, "password", cfg.HttpAuth.Password)
	assert.Equal(t, true, cfg.TLSConfig.InsecureSkipVerify)
}
