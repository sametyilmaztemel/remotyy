package main

import (
	"testing"
)

func TestSplitHostName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-host linux/arm64 terminal", "my-host"},
		{"simple", "simple"},
		{"", ""},
		{"host with spaces", "host"},
	}

	for _, tt := range tests {
		got := splitHostName(tt.input)
		if got != tt.want {
			t.Errorf("splitHostName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestJoinStringsTUI(t *testing.T) {
	tests := []struct {
		input []string
		sep   string
		want  string
	}{
		{[]string{"terminal", "screen", "file"}, ", ", "terminal, screen, file"},
		{[]string{"a"}, ",", "a"},
		{[]string{}, ", ", ""},
	}

	for _, tt := range tests {
		got := joinStrings(tt.input, tt.sep)
		if got != tt.want {
			t.Errorf("joinStrings(%v, %q) = %q, want %q", tt.input, tt.sep, got, tt.want)
		}
	}
}

func TestInitialModel(t *testing.T) {
	m := initialModel()
	if m.state != "init" {
		t.Errorf("initial state = %q, want init", m.state)
	}
	if m.signalURL != "ws://localhost:9000" {
		t.Errorf("initial signalURL = %q", m.signalURL)
	}
	if m.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", m.cursor)
	}
}

func TestModelViewInit(t *testing.T) {
	m := initialModel()
	view := m.View()
	if len(view) == 0 {
		t.Error("View should produce output")
	}
}
