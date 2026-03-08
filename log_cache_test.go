package main

import (
	"fmt"
	"testing"
)

func TestLogCache(t *testing.T) {
	size := 5
	lc := newLogCache(size)

	// Test writing fewer than size
	for i := 0; i < 3; i++ {
		fmt.Fprintf(lc, "log message %d\n", i)
	}

	logs := lc.GetLogs()
	if len(logs) != 3 {
		t.Errorf("expected 3 logs, got %d", len(logs))
	}
	if logs[0] != "log message 0" {
		t.Errorf("expected 'log message 0', got '%s'", logs[0])
	}

	// Test filling the cache
	for i := 3; i < 5; i++ {
		fmt.Fprintf(lc, "log message %d\n", i)
	}
	logs = lc.GetLogs()
	if len(logs) != 5 {
		t.Errorf("expected 5 logs, got %d", len(logs))
	}

	// Test overflowing the cache
	fmt.Fprintf(lc, "log message 5\n")
	logs = lc.GetLogs()
	if len(logs) != 5 {
		t.Errorf("expected 5 logs, got %d", len(logs))
	}
	// Should contain messages 1 to 5
	if logs[0] != "log message 1" {
		t.Errorf("expected 'log message 1', got '%s'", logs[0])
	}
	if logs[4] != "log message 5" {
		t.Errorf("expected 'log message 5', got '%s'", logs[4])
	}

	// Test Write method directly
	n, err := lc.Write([]byte("test write"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 10 {
		t.Errorf("expected 10 bytes written, got %d", n)
	}
}
