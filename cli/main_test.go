package main

import (
	"testing"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		arg          string
		wantHost     string
		wantDeckName string
		wantRemote   bool
	}{
		{"deck.md", "", "deck", false},
		{"path/to/deck.md", "", "deck", false},
		{"server:deck.md", "server", "deck", true},
		{"192.168.1.1:deck.md", "192.168.1.1", "deck", true},
		{"host:path/to/deck.md", "host", "deck", true},
		{"host:deck", "host", "deck", true},
		{"nodot", "", "nodot", false},
	}
	for _, tt := range tests {
		host, deckName, isRemote := parseTarget(tt.arg)
		if host != tt.wantHost {
			t.Errorf("parseTarget(%q) host = %q, want %q", tt.arg, host, tt.wantHost)
		}
		if deckName != tt.wantDeckName {
			t.Errorf("parseTarget(%q) deckName = %q, want %q", tt.arg, deckName, tt.wantDeckName)
		}
		if isRemote != tt.wantRemote {
			t.Errorf("parseTarget(%q) isRemote = %v, want %v", tt.arg, isRemote, tt.wantRemote)
		}
	}
}

func TestDeckNameFromPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"deck.md", "deck"},
		{"path/to/deck.md", "deck"},
		{"noext", "noext"},
		{"/abs/path/name.db", "name"},
	}
	for _, tt := range tests {
		got := deckNameFromPath(tt.in)
		if got != tt.want {
			t.Errorf("deckNameFromPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
