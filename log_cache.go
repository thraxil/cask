package main

import (
	"sync"
)

type LogCache struct {
	mu   sync.Mutex
	logs []string
	size int
}

func newLogCache(size int) *LogCache {
	return &LogCache{
		size: size,
		logs: make([]string, 0, size),
	}
}

func (lc *LogCache) Write(p []byte) (n int, err error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	msg := string(p)
	// Remove trailing newline if present for cleaner display
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	lc.logs = append(lc.logs, msg)
	if len(lc.logs) > lc.size {
		// Keep the last `size` elements
		lc.logs = lc.logs[len(lc.logs)-lc.size:]
	}

	return len(p), nil
}

func (lc *LogCache) GetLogs() []string {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	// Return a copy
	result := make([]string, len(lc.logs))
	copy(result, lc.logs)
	return result
}
