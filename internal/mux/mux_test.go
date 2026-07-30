package mux

import (
	"testing"
)

func TestNewMultiplexer(t *testing.T) {
	m := NewMultiplexer()
	if m == nil {
		t.Fatal("NewMultiplexer returned nil")
	}
}

func TestOpenChannel(t *testing.T) {
	m := NewMultiplexer()
	ch, err := m.OpenChannel(ChanTerminal)
	if err != nil {
		t.Fatalf("OpenChannel: %v", err)
	}
	if ch == nil {
		t.Error("channel should not be nil")
	}
}

func TestOpenChannelDuplicate(t *testing.T) {
	m := NewMultiplexer()
	m.OpenChannel(ChanTerminal)
	_, err := m.OpenChannel(ChanTerminal)
	if err == nil {
		t.Error("duplicate channel should return error")
	}
}

func TestOpenMultipleChannels(t *testing.T) {
	m := NewMultiplexer()
	channels := []ChannelID{ChanTerminal, ChanScreen, ChanFile, ChanClipboard, ChanKeepalive}
	for _, id := range channels {
		_, err := m.OpenChannel(id)
		if err != nil {
			t.Errorf("OpenChannel(%d): %v", id, err)
		}
	}
}

func TestWriteTo(t *testing.T) {
	m := NewMultiplexer()
	ch, _ := m.OpenChannel(ChanTerminal)

	err := m.WriteTo(ChanTerminal, []byte("hello"))
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	data := <-ch
	if string(data) != "hello" {
		t.Errorf("received %q, want %q", string(data), "hello")
	}
}

func TestWriteToNonExistent(t *testing.T) {
	m := NewMultiplexer()
	err := m.WriteTo(ChanTerminal, []byte("hello"))
	if err == nil {
		t.Error("WriteTo to non-existent channel should return error")
	}
}

func TestClose(t *testing.T) {
	m := NewMultiplexer()
	m.OpenChannel(ChanTerminal)
	m.Close(ChanTerminal)

	// WriteTo should fail after close
	err := m.WriteTo(ChanTerminal, []byte("hello"))
	if err == nil {
		t.Error("WriteTo after close should return error")
	}
}

func TestCloseNonExistent(t *testing.T) {
	m := NewMultiplexer()
	// Should not panic
	m.Close(ChanTerminal)
}

func TestWriteToBufferFull(t *testing.T) {
	m := NewMultiplexer()
	// Channel has buffer of 100
	ch, _ := m.OpenChannel(ChanTerminal)

	// Fill the buffer
	for i := 0; i < 100; i++ {
		m.WriteTo(ChanTerminal, []byte("x"))
	}

	// One more write should not error (drops silently)
	err := m.WriteTo(ChanTerminal, []byte("overflow"))
	if err != nil {
		t.Errorf("WriteTo on full buffer should drop silently, got error: %v", err)
	}

	// Drain and verify
	drained := 0
	for {
		select {
		case <-ch:
			drained++
		default:
			goto done
		}
	}
done:
	if drained != 100 {
		t.Errorf("drained %d messages, want 100", drained)
	}
}

func TestConcurrentWrite(t *testing.T) {
	m := NewMultiplexer()
	ch, _ := m.OpenChannel(ChanTerminal)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.WriteTo(ChanTerminal, []byte("data"))
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic, channel should have data
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			return
		}
	}
}
