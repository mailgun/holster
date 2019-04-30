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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	leader := make(chan bool, 5)
	defer cancel()

	logrus.SetLevel(logrus.DebugLevel)

	election, err := etcdutil.NewElection(ctx, client, etcdutil.ElectionConfig{
		ElectionObserver: func(isLeader bool, candidate string, err error) {
			if err != nil {
				t.Fatal(err.Error())
			}
			leader <- isLeader
		},
		Election:  "/my-election",
		Candidate: "me",
	})
	require.Nil(t, err)

	assert.Equal(t, true, election.IsLeader())

	// TODO: Close()
}
