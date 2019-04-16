package etcdutil_test

import (
	"context"
	"testing"
	"time"

	"github.com/mailgun/holster/etcdutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestElection(t *testing.T) {
	client, err := etcdutil.NewClient(nil)
	require.Nil(t, err)

	election := etcdutil.NewElection(client, etcdutil.ElectionConfig{
		Log:       logrus.WithField("category", "election"),
		Election:  "/my-election",
		Candidate: "me",
	})

	election.Register(func(e etcdutil.ElectionEvent) {
		switch e.Event {
		case etcdutil.EventGainLeader:
			// I'm leader!
		case etcdutil.EventLostLeader:
			// I'm NOT leader!
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	err = election.Start(ctx)
	require.Nil(t, err)

	assert.Equal(t, true, election.IsLeader())
}
