package syncutil_test

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/mailgun/holster/v4/syncutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayloadBroadcast(t *testing.T) {
	const numMessages = 5
	const numClients = 2
	broadcaster := syncutil.NewPayloadBroadcaster()
	socket := make(chan string)
	var readyWg, doneWg sync.WaitGroup

	// Start some clients that will receive broadcast messages.
	for i := 0; i < numClients; i++ {
		readyWg.Add(1)
		doneWg.Add(1)

		go func(idx int) {
			var counter int
			broadcastChan := broadcaster.WaitChan(strconv.Itoa(idx))
			readyWg.Done()

			for {
				expectedPayload := interface{}(fmt.Sprintf("Foobar payload %d", counter))

				// Wait for more payloads.
				select {
				case payload, ok := <-broadcastChan:
					require.True(t, ok)

					if payload == nil {
						// EOF
						t.Logf("Client[%d] EOF", idx)
						doneWg.Done()
						return
					}

					// Verify payload content.
					assert.Equal(t, expectedPayload, payload)
					t.Logf("Client[%d] received payload: %#v", idx, payload)
					socket <- fmt.Sprintf("Client[%d] Payload: %#v\n", idx, payload)
				}

				counter++
				require.LessOrEqual(t, counter, numMessages)
			}
		}(i)
	}

	// Wait for the clients to be ready
	readyWg.Wait()

	// Channel reader.
	// Accumulate output from all clients.
	var messageCount int
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)

		for payload := range socket {
			t.Logf("Received payload: %#v", payload)
			messageCount++
		}
	}()

	// Broadcast some messages.
	for i := 0; i < numMessages; i++ {
		sendPayload := fmt.Sprintf("Foobar payload %d", i)
		t.Logf("Sending payload: %#v", sendPayload)
		broadcaster.Broadcast(sendPayload)
	}
	broadcaster.Broadcast(nil)

	// Wait for all the things to finish.
	doneWg.Wait()
	close(socket)
	<-readerDone

	// Verify total message count.
	expectedCount := numMessages * numClients
	assert.Equal(t, expectedCount, messageCount)
}
