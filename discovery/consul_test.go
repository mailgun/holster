package discovery_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/mailgun/holster/v4/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func printCatalog(t *testing.T, catalog string, client *api.Client) {
	t.Helper()
	fmt.Printf("============\n")
	l, _, err := client.Health().Service(catalog, "", false, nil)
	require.NoError(t, err)
	for _, i := range l {
		t.Logf("Service: %s", i.Service.ID)
	}
	fmt.Printf("======\n")
}

func TestConsulSinglePeer(t *testing.T) {
	const catalog = "TestConsulSinglePeer"
	p := discovery.Peer{ID: "id-1", Metadata: []byte("address-0"), IsSelf: true}

	//client, err := api.NewClient(api.DefaultConfig())
	//require.NoError(t, err)

	//printCatalog(t, catalog, client)

	onUpdateCh := make(chan []discovery.Peer, 1)
	cs, err := discovery.NewConsul(&discovery.ConsulConfig{
		CatalogName: catalog,
		Peer:        p,
		OnUpdate: func(peers []discovery.Peer) {
			onUpdateCh <- peers
		},
	})
	//printCatalog(t, catalog, client)

	e := <-onUpdateCh
	assert.Equal(t, p, e[0])

	err = cs.Close(context.Background())
	require.NoError(t, err)

	//printCatalog(t, catalog, client)
}

func TestConsulMultiplePeers(t *testing.T) {
	const catalog = "TestConsulMultiplePeers"
	p0 := discovery.Peer{ID: "id-0", Metadata: []byte("address-0"), IsSelf: true}
	p1 := discovery.Peer{ID: "id-1", Metadata: []byte("address-1")}

	//client, err := api.NewClient(api.DefaultConfig())
	//require.NoError(t, err)

	//printCatalog(t, catalog, client)

	onUpdateCh := make(chan []discovery.Peer, 2)
	cs0, err := discovery.NewConsul(&discovery.ConsulConfig{
		CatalogName: catalog,
		Peer:        p0,
		OnUpdate: func(peers []discovery.Peer) {
			onUpdateCh <- peers
		},
	})
	require.NoError(t, err)
	defer cs0.Close(context.Background())

	e := <-onUpdateCh
	assert.Equal(t, e[0], p0)

	//printCatalog(t, catalog, client)

	cs1, err := discovery.NewConsul(&discovery.ConsulConfig{
		CatalogName: catalog,
		Peer:        p1,
	})
	require.NoError(t, err)
	defer cs1.Close(context.Background())

	e = <-onUpdateCh
	assert.Equal(t, []discovery.Peer{p0, p1}, e)

	//printCatalog(t, catalog, client)
}
