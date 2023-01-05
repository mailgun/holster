package discovery_test

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/mailgun/holster/v4/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSRVAddressesDirect(t *testing.T) {
	client, err := api.NewClient(api.DefaultConfig())
	require.NoError(t, err)

	// Register ourselves in consul as a member of the cluster
	const id = "1234123"
	err = client.Agent().ServiceRegisterOpts(&api.AgentServiceRegistration{
		Name:    "scout",
		ID:      id,
		Tags:    []string{"ml"},
		Address: "127.0.0.1",
		Port:    2319,
	}, api.ServiceRegisterOpts{ReplaceExistingChecks: true})
	require.NoError(t, err)
	defer func() {
		err := client.Agent().ServiceDeregister(id)
		require.NoError(t, err)
	}()

	addresses, err := discovery.GetSRVAddresses("ml.scout.service.consul", "127.0.0.1:8600")
	require.NoError(t, err)

	assert.Equal(t, []string{"127.0.0.1:2319"}, addresses)
}

func TestGetSRVAddresses(t *testing.T) {
	t.Skip("TODO: fix https://github.com/mailgun/holster/issues/152")

	client, err := api.NewClient(api.DefaultConfig())
	require.NoError(t, err)

	// Register ourselves in consul as a member of the cluster
	err = client.Agent().ServiceRegisterOpts(&api.AgentServiceRegistration{
		Name:    "scout",
		ID:      "123-2319",
		Tags:    []string{"mll"},
		Address: "127.0.0.1",
		Port:    2319,
	}, api.ServiceRegisterOpts{ReplaceExistingChecks: true})
	require.NoError(t, err)
	defer func() {
		err := client.Agent().ServiceDeregister("123-2319")
		require.NoError(t, err)
	}()

	addresses, err := discovery.GetSRVAddresses("mll.scout.service.consul", "")
	require.NoError(t, err)

	assert.Equal(t, []string{"127.0.0.1:2319"}, addresses)
}
