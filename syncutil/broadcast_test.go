package syncutil_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mailgun/holster/v4/syncutil"
	"github.com/stretchr/testify/assert"
)

func TestBroadcaster(t *testing.T) {
	t.Run("Happy path", func(t *testing.T) {
		broadcaster := syncutil.NewBroadcaster()
		ready := make(chan struct{}, 2)
		done := make(chan struct{})
		socket := make(chan string, 11)
		var mutex sync.Mutex
		var chat []string

		// Start some simple chat clients that are responsible for
		// sending the contents of the []chat slice to their clients
		for i := 0; i < 2; i++ {
			go func(idx int) {
				var clientIndex int
				var once sync.Once
				for {
					mutex.Lock()
					if clientIndex != len(chat) {
						// Pretend we are sending a message to our client via a socket
						socket <- fmt.Sprintf("Client [%d] Chat: %s\n", idx, chat[clientIndex])
						clientIndex++
						mutex.Unlock()
						continue
					}
					mutex.Unlock()

					// Indicate the client is up and ready to receive broadcasts
					once.Do(func() {
						ready <- struct{}{}
					})

					// Wait for more chats to be added to chat[]
					select {
					case <-broadcaster.WaitChan(fmt.Sprint(idx)):
					case <-done:
						return
					}
				}
			}(i)
		}

		// Wait for the clients to be ready
		<-ready
		<-ready

		// Add some chat lines to the []chat slice
		for i := 0; i < 5; i++ {
			mutex.Lock()
			chat = append(chat, fmt.Sprintf("Message '%d'", i))
			mutex.Unlock()

			// Notify any clients there are new chats to read
			broadcaster.Broadcast()
		}

		var count int
		for msg := range socket {
			fmt.Printf(msg)
			count++
			if count == 10 {
				break
			}
		}

		if count != 10 {
			t.Errorf("count != 10")
		}
		// Tell the clients to quit
		close(done)
	})

	t.Run("Remove()", func(t *testing.T) {
		const client1 = "Foobar1"
		const client2 = "Foobar2"

		t.Run("When using WaitChan()", func(t *testing.T) {
			broadcaster := syncutil.NewBroadcaster()

			// Register broadcast clients with WaitChan().
			ch1 := broadcaster.WaitChan(client1)
			ch2 := broadcaster.WaitChan(client2)

			// Test broadcast.
			broadcaster.Broadcast()
			assert.Len(t, ch1, 1)
			assert.Len(t, ch2, 1)
			<-ch1
			<-ch2

			// Remove a client.
			broadcaster.Remove(client1)

			// Test broadcast again.
			// Verify client1 channel is unchanged.
			broadcaster.Broadcast()
			assert.Empty(t, ch1)
			assert.Len(t, ch2, 1)
		})

		t.Run("When using Wait()", func(t *testing.T) {
			var doneWg sync.WaitGroup
			broadcaster := syncutil.NewBroadcaster()

			// Register broadcast clients with Wait().
			doneWg.Add(2)
			var c1Count, c2Count int

			go func() {
				defer doneWg.Done()
				broadcaster.Wait(client1)
				c1Count++
			}()
			go func() {
				defer doneWg.Done()
				broadcaster.Wait(client2)
				c2Count++
			}()

			for {
				time.Sleep(1 * time.Millisecond)
				if !broadcaster.Has(client1) {
					continue
				}
				if !broadcaster.Has(client2) {
					continue
				}
				break
			}

			// Test broadcast.
			broadcaster.Broadcast()
			doneWg.Wait()
			assert.Equal(t, 1, c1Count)
			assert.Equal(t, 1, c2Count)

			// Get the generated channels.
			ch1 := broadcaster.WaitChan(client1)
			ch2 := broadcaster.WaitChan(client2)

			// Remove a client.
			broadcaster.Remove(client1)

			// Test broadcast again.
			// Verify client1 channel is unchanged.
			broadcaster.Broadcast()
			assert.Empty(t, ch1)
			assert.Len(t, ch2, 1)
		})
	})

	t.Run("WithChannelSize()", func(t *testing.T) {
		const expectedChannelSize = 0xc0ffee
		broadcaster := syncutil.NewBroadcaster(syncutil.WithChannelSize(expectedChannelSize))
		ch := broadcaster.WaitChan("Foobar")
		size := cap(ch)
		assert.Equal(t, expectedChannelSize, size)
	})
}
