package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelInit(t *testing.T) {
	m := initialModel()
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a command")
	}
}

func TestModelUpdateQuit(t *testing.T) {
	m := initialModel()

	// Test 'q' quits
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected quit command for 'q' key")
	}
	_, ok := result.(model)
	if !ok {
		t.Error("result should be a model")
	}
}

func TestModelUpdateCtrlC(t *testing.T) {
	m := initialModel()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected quit command for ctrl+c")
	}
}

func TestModelUpdateCursorUp(t *testing.T) {
	m := initialModel()
	m.state = "hosts"
	m.hosts = []string{"host1", "host2", "host3"}
	m.cursor = 2 // start at bottom

	// Move up with 'k'
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	newModel := result.(model)
	if newModel.cursor != 1 {
		t.Errorf("cursor after up = %d, want 1", newModel.cursor)
	}

	// Move up again with "up" key
	result, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyUp})
	newModel = result.(model)
	if newModel.cursor != 0 {
		t.Errorf("cursor after second up = %d, want 0", newModel.cursor)
	}

	// Should not go below 0
	result, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyUp})
	newModel = result.(model)
	if newModel.cursor != 0 {
		t.Errorf("cursor at top = %d, want 0", newModel.cursor)
	}
}

func TestModelUpdateCursorDown(t *testing.T) {
	m := initialModel()
	m.state = "hosts"
	m.hosts = []string{"host1", "host2", "host3"}
	m.cursor = 0

	// Move down with 'j'
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	newModel := result.(model)
	if newModel.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", newModel.cursor)
	}

	// Move down with "down" key
	result, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyDown})
	newModel = result.(model)
	if newModel.cursor != 2 {
		t.Errorf("cursor after second down = %d, want 2", newModel.cursor)
	}

	// Should not go past end
	result, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyDown})
	newModel = result.(model)
	if newModel.cursor != 2 {
		t.Errorf("cursor at bottom = %d, want 2", newModel.cursor)
	}
}

func TestModelUpdateEnterWithHosts(t *testing.T) {
	m := initialModel()
	m.state = "hosts"
	m.hosts = []string{"host1", "host2"}
	m.cursor = 0

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(model)

	if newModel.state != "connecting" {
		t.Errorf("state after enter = %q, want %q", newModel.state, "connecting")
	}
	if !strings.Contains(newModel.status, "host1") {
		t.Errorf("status should mention selected host, got %q", newModel.status)
	}
	if cmd == nil {
		t.Error("expected connect command")
	}
}

func TestModelUpdateEnterNoHosts(t *testing.T) {
	m := initialModel()
	m.state = "hosts"
	m.hosts = []string{} // no hosts

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(model)

	if newModel.state != "hosts" {
		t.Errorf("state = %q, want %q", newModel.state, "hosts")
	}
	if cmd != nil {
		t.Error("no command expected when no hosts")
	}
}

func TestModelUpdateRefresh(t *testing.T) {
	m := initialModel()
	m.state = "hosts"

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	newModel := result.(model)

	if newModel.status != "Refreshing..." {
		t.Errorf("status = %q, want %q", newModel.status, "Refreshing...")
	}
	if cmd == nil {
		t.Error("expected listHosts command")
	}
}

func TestModelUpdateHostsMsg(t *testing.T) {
	m := initialModel()

	result, _ := m.Update(hostsMsg{"host1", "host2", "host3"})
	newModel := result.(model)

	if newModel.state != "hosts" {
		t.Errorf("state = %q, want %q", newModel.state, "hosts")
	}
	if len(newModel.hosts) != 3 {
		t.Errorf("hosts count = %d, want 3", len(newModel.hosts))
	}
	if !strings.Contains(newModel.status, "3 host(s)") {
		t.Errorf("status = %q, should mention 3 hosts", newModel.status)
	}
}

func TestModelUpdateStatusMsg(t *testing.T) {
	m := initialModel()

	result, _ := m.Update(statusMsg("custom status"))
	newModel := result.(model)

	if newModel.status != "custom status" {
		t.Errorf("status = %q, want %q", newModel.status, "custom status")
	}
}

func TestModelUpdateErrorMsg(t *testing.T) {
	m := initialModel()

	result, _ := m.Update(errorMsg("something went wrong"))
	newModel := result.(model)

	if !strings.Contains(newModel.status, "something went wrong") {
		t.Errorf("status = %q, should contain error", newModel.status)
	}
}

func TestModelViewHosts(t *testing.T) {
	m := initialModel()
	m.state = "hosts"
	m.hosts = []string{"host1 linux/arm64 terminal", "host2 darwin/amd64 screen"}
	m.status = "2 host(s) available"

	view := m.View()
	if !strings.Contains(view, "host1") {
		t.Error("view should show host1")
	}
	if !strings.Contains(view, "host2") {
		t.Error("view should show host2")
	}
	if !strings.Contains(view, "navigate") {
		t.Error("view should show navigation hints")
	}
}

func TestModelViewSelectedHost(t *testing.T) {
	m := initialModel()
	m.state = "hosts"
	m.hosts = []string{"selected-host", "other-host"}
	m.cursor = 0

	view := m.View()

	if !strings.Contains(view, "▸") {
		t.Error("view should have cursor marker on selected host")
	}
}

func TestModelViewConnecting(t *testing.T) {
	m := initialModel()
	m.state = "connecting"
	m.status = "Connecting to host1..."
	m.hosts = []string{"host1"}

	view := m.View()
	if !strings.Contains(view, "Connecting") {
		t.Error("connecting view should show status")
	}
}
