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