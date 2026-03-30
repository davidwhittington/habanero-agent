package engine

import (
	"testing"
	"time"
)

func TestDNSResolve(t *testing.T) {
	addr, duration, err := DNSResolve("localhost", "")
	if err != nil {
		t.Skipf("DNS resolution not available: %v", err)
	}

	if addr == "" {
		t.Error("expected non-empty address")
	}
	if duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestTCPConnect(t *testing.T) {
	// Test connection to a likely-closed port on localhost.
	open, duration := TCPConnect("127.0.0.1", 1, 2*time.Second)
	if open {
		t.Skip("port 1 unexpectedly open on localhost")
	}
	if duration == 0 {
		t.Error("expected non-zero duration even for failed connection")
	}
}

func TestPingResultDefaults(t *testing.T) {
	// Test that PingResult is zero-valued correctly.
	r := PingResult{}
	if r.Host != "" {
		t.Error("expected empty host")
	}
	if r.Loss != 0 {
		t.Error("expected zero loss")
	}
}
