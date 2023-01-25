package discovery_test

import (
	"context"
	"testing"

	"github.com/mailgun/holster/v5/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemberListMultiplePeers(t *testing.T) {
	p0 := discovery.Peer{ID: "id-0", Metadata: []byte("address-0"), IsSelf: true}
	p1 := discovery.Peer{ID: "id-1", Metadata: []byte("address-1")}

	onUpdateCh := make(chan []discovery.Peer, 2)
	ml0, err := discovery.NewMemberList(context.Background(), discovery.MemberListConfig{
		BindAddress: "localhost:8519",
		Peer:        p0,
		OnUpdate: func(peers []discovery.Peer) {
			onUpdateCh <- peers
		},
	})
	require.NoError(t, err)
	defer ml0.Close(context.Background())

	e := <-onUpdateCh
	assert.Equal(t, e[0], p0)

	ml1, err := discovery.NewMemberList(context.Background(), discovery.MemberListConfig{
		KnownPeers:  []string{"localhost:8519"},
		BindAddress: "localhost:8518",
		Peer:        p1,
	})
	require.NoError(t, err)
	defer ml1.Close(context.Background())

	e = <-onUpdateCh
	assert.Equal(t, []discovery.Peer{p0, p1}, e)
}
