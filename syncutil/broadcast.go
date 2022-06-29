/*
Copyright 2022 Mailgun Technologies Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package syncutil

import "sync"

type Broadcaster interface {
	WaitChan(string) chan struct{}
	Wait(string)
	Broadcast()
	Has(string) bool
	Remove(string)
	Done()
}

// Broadcasts to goroutines a new event has occurred and any waiting go routines should
// stop waiting and do work. The current implementation is limited to 10,0000 unconsumed
// broadcasts. If the user broadcasts more events than can be consumed calls to broadcast()
// will eventually block until the goroutines can catch up. This ensures goroutines will
// receive at least one event per broadcast() call.
type broadcast struct {
	clients     map[string]chan struct{}
	done        chan struct{}
	mutex       sync.Mutex
	channelSize int
}

type BroadcasterOption interface {
	Apply(*broadcast)
}

const DefaultChannelSize = 10000

func NewBroadcaster(opts ...BroadcasterOption) Broadcaster {
	br := &broadcast{
		clients:     make(map[string]chan struct{}),
		done:        make(chan struct{}),
		channelSize: DefaultChannelSize,
	}

	for _, opt := range opts {
		opt.Apply(br)
	}

	return br
}

// Notify all Waiting goroutines
func (b *broadcast) Broadcast() {
	b.mutex.Lock()
	for _, channel := range b.clients {
		channel <- struct{}{}
	}
	b.mutex.Unlock()
}

// Cancels any Wait() calls that are currently blocked
func (b *broadcast) Done() {
	close(b.done)
}

// Blocks until a broadcast is received
func (b *broadcast) Wait(name string) {
	b.mutex.Lock()
	channel, ok := b.clients[name]
	if !ok {
		b.clients[name] = make(chan struct{}, b.channelSize)
		channel = b.clients[name]
	}
	b.mutex.Unlock()

	// Wait for a new event or done is closed
	select {
	case <-channel:
		return
	case <-b.done:
		return
	}
}

// Returns a channel the caller can use to wait for a broadcast
func (b *broadcast) WaitChan(name string) chan struct{} {
	b.mutex.Lock()
	channel, ok := b.clients[name]
	if !ok {
		b.clients[name] = make(chan struct{}, b.channelSize)
		channel = b.clients[name]
	}
	b.mutex.Unlock()
	return channel
}

// Has checks if a client name is registered.
func (b *broadcast) Has(name string) bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	_, exists := b.clients[name]
	return exists
}

// Remove client name previously registered by Wait/WaitChan.
func (b *broadcast) Remove(name string) {
	b.mutex.Lock()
	delete(b.clients, name)
	b.mutex.Unlock()
}

type withChannelSizeOption struct {
	channelSize int
}

// WithChannelSize sets the client's broadcast channel size.
func WithChannelSize(channelSize int) BroadcasterOption {
	return &withChannelSizeOption{channelSize: channelSize}
}

func (o *withChannelSizeOption) Apply(b *broadcast) {
	b.channelSize = o.channelSize
}
