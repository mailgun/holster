package syncutil_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/mailgun/holster/v3/syncutil"
)

func TestBroadcast(t *testing.T) {
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
}
