package audit

import (
	"context"
	"sync"
	"testing"
)

func TestWithIP(t *testing.T) {
	ctx := context.Background()
	ip := ipFromContext(ctx)
	if ip != "" {
		t.Fatalf("expected empty ip, got %q", ip)
	}

	ctx = WithIP(ctx, "192.168.1.1")
	ip = ipFromContext(ctx)
	if ip != "192.168.1.1" {
		t.Fatalf("expected 192.168.1.1, got %q", ip)
	}
}

func TestLogWithNilQueries(t *testing.T) {
	// Logger with nil queries should still log to slog without panic on the sync path.
	// The async drain will fail on DB insert but that's expected — it logs an error.
	// We just verify the Log call itself doesn't panic.
	l := &Logger{
		queries: nil,
		ch:      make(chan entry, 16),
	}

	// Log should not panic even with nil queries (slog path is sync)
	l.Log(context.Background(), "test", "test.action", "org_1", "target_1", map[string]any{"key": "val"})

	// Verify the entry was queued
	select {
	case e := <-l.ch:
		if e.action != "test.action" {
			t.Fatalf("expected action test.action, got %q", e.action)
		}
		if e.orgID != "org_1" {
			t.Fatalf("expected org_id org_1, got %q", e.orgID)
		}
		if e.target != "target_1" {
			t.Fatalf("expected target target_1, got %q", e.target)
		}
	default:
		t.Fatal("expected entry in channel")
	}
}

func TestLogChannelFull(t *testing.T) {
	// Test that Log doesn't block when channel is full
	l := &Logger{
		queries: nil,
		ch:      make(chan entry, 1),
	}

	// Fill the channel
	l.Log(context.Background(), "a", "first", "", "", nil)
	// This should not block — it drops the event
	l.Log(context.Background(), "a", "second", "", "", nil)

	// Only the first should be in the channel
	select {
	case e := <-l.ch:
		if e.action != "first" {
			t.Fatalf("expected first, got %q", e.action)
		}
	default:
		t.Fatal("expected entry")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	l := New(nil, 16)
	l.Close()
	l.Close() // second call must not panic
}

func TestLogAfterClose(t *testing.T) {
	// After Close(), Log must not panic (no send-on-closed-channel).
	l := New(nil, 16)
	l.Close()
	// This must not panic
	l.Log(context.Background(), "test", "test.action", "", "", nil)
}

func TestConcurrentLogAndClose(t *testing.T) {
	// Hammer Log() and Close() concurrently to detect races.
	// Run with -race to verify.
	for i := 0; i < 100; i++ {
		l := New(nil, 4)
		var wg sync.WaitGroup

		// Spawn writers
		for j := 0; j < 10; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for k := 0; k < 50; k++ {
					l.Log(context.Background(), "a", "spam", "", "", nil)
				}
			}()
		}

		// Close mid-flight
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Close()
		}()

		wg.Wait()
	}
}

func TestIPFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected string
	}{
		{"ipv4", "10.0.0.1", "10.0.0.1"},
		{"ipv6", "::1", "::1"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.ip != "" {
				ctx = WithIP(ctx, tt.ip)
			}
			got := ipFromContext(ctx)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
