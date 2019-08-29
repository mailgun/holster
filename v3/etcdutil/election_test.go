package etcdutil_test

import (
	"context"
	"testing"
	"time"

	"github.com/mailgun/holster/v3/etcdutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestElection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	election, err := etcdutil.NewElection(ctx, client, etcdutil.ElectionConfig{
		EventObserver: func(e etcdutil.Event) {
			if e.Err != nil {
				t.Fatal(e.Err.Error())
			}
		},
		Election:  "/my-election",
		Candidate: "me",
	})
	require.Nil(t, err)

	assert.Equal(t, true, election.IsLeader())
	election.Close()
	assert.Equal(t, false, election.IsLeader())
}

func TestTwoCampaigns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	logrus.SetLevel(logrus.DebugLevel)

	c1, err := etcdutil.NewElection(ctx, client, etcdutil.ElectionConfig{
		EventObserver: func(e etcdutil.Event) {
			if e.Err != nil {
				t.Fatal(e.Err.Error())
			}
		},
		Election:  "/my-election",
		Candidate: "c1",
	})
	require.Nil(t, err)

	c2Chan := make(chan etcdutil.Event, 5)
	c2, err := etcdutil.NewElection(ctx, client, etcdutil.ElectionConfig{
		EventObserver: func(e etcdutil.Event) {
			if err != nil {
				t.Fatal(err.Error())
			}
			c2Chan <- e
		},
		Election:  "/my-election",
		Candidate: "c2",
	})
	require.Nil(t, err)

	assert.Equal(t, true, c1.IsLeader())
	assert.Equal(t, false, c2.IsLeader())

	// Cancel first candidate
	c1.Close()
	assert.Equal(t, false, c1.IsLeader())

	// Second campaign should become leader
	e := <-c2Chan
	assert.Equal(t, false, e.IsLeader)
	e = <-c2Chan
	assert.Equal(t, true, e.IsLeader)
	assert.Equal(t, false, e.IsDone)

	c2.Close()
	e = <-c2Chan
	assert.Equal(t, false, e.IsLeader)
	assert.Equal(t, false, e.IsDone)

	e = <-c2Chan
	assert.Equal(t, false, e.IsLeader)
	assert.Equal(t, true, e.IsDone)
}
