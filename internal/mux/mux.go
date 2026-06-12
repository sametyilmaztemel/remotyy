// Package mux provides connection multiplexing for remotty.
package mux

import (
	"fmt"
	"sync"
)

// ChannelID identifies a multiplexed channel.
type ChannelID uint16

const (
	ChanTerminal ChannelID = 1
	ChanScreen   ChannelID = 2
	ChanFile     ChannelID = 3
	ChanClipboard ChannelID = 4
	ChanKeepalive ChannelID = 5
)

// Multiplexer allows multiple logical channels over a single WebRTC data channel.
type Multiplexer struct {
	mu         sync.Mutex
	listeners  map[ChannelID]chan []byte
	nextID     ChannelID
}

// NewMultiplexer creates a new multiplexer.
func NewMultiplexer() *Multiplexer {
	return &Multiplexer{
		listeners: make(map[ChannelID]chan []byte),
		nextID:    10,
	}
}

// OpenChannel creates a new channel with the given ID.
func (m *Multiplexer) OpenChannel(id ChannelID) (chan []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.listeners[id]; ok {
		return nil, fmt.Errorf("channel %d already exists", id)
	}

	ch := make(chan []byte, 100)
	m.listeners[id] = ch
	return ch, nil
}

// WriteTo sends data to a specific channel.
func (m *Multiplexer) WriteTo(id ChannelID, data []byte) error {
	m.mu.Lock()
	ch, ok := m.listeners[id]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("channel %d not found", id)
	}

	select {
	case ch <- data:
	default:
		// Drop if full
	}
	return nil
}

// Close removes a channel.
func (m *Multiplexer) Close(id ChannelID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.listeners, id)
}
