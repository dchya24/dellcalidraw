package websocket

import (
	"sync"
	"time"
)

// RateLimiter limits message frequency per connection
type RateLimiter struct {
	mu              sync.RWMutex
	lastMessageTime map[string]time.Time // connID -> last message time
	messageCount    map[string]int       // connID -> message count in window
	windowStart     map[string]time.Time // connID -> window start time

	maxMessagesPerSecond int
	maxMessagesPerWindow int
	windowDuration       time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		lastMessageTime:     make(map[string]time.Time),
		messageCount:        make(map[string]int),
		windowStart:         make(map[string]time.Time),
		maxMessagesPerSecond: 20, // Allow 20 messages per second
		maxMessagesPerWindow: 100, // Allow 100 messages per 10 seconds
		windowDuration:      10 * time.Second,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a message is allowed for the given connection
func (rl *RateLimiter) Allow(connID string) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Check per-second limit
	if lastTime, exists := rl.lastMessageTime[connID]; exists {
		elapsed := now.Sub(lastTime)
		if elapsed < time.Second/20 { // More than 20 messages per second
			return false
		}
	}

	// Check per-window limit
	windowStart, windowExists := rl.windowStart[connID]
	if !windowExists || now.Sub(windowStart) > rl.windowDuration {
		// Start new window
		rl.windowStart[connID] = now
		rl.messageCount[connID] = 1
		rl.lastMessageTime[connID] = now
		return true
	}

	// Check if window limit exceeded
	if rl.messageCount[connID] >= rl.maxMessagesPerWindow {
		return false
	}

	// Increment counters
	rl.messageCount[connID]++
	rl.lastMessageTime[connID] = now

	return true
}

// cleanup removes stale entries from rate limiter maps
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()

		for connID, lastTime := range rl.lastMessageTime {
			// Remove entries not used in the last 5 minutes
			if now.Sub(lastTime) > 5*time.Minute {
				delete(rl.lastMessageTime, connID)
				delete(rl.messageCount, connID)
				delete(rl.windowStart, connID)
			}
		}

		rl.mu.Unlock()
	}
}

// CursorRateLimiter is specifically for cursor updates (higher frequency allowed)
type CursorRateLimiter struct {
	mu              sync.RWMutex
	lastUpdateTime  map[string]time.Time
	maxUpdatesPerSecond int
}

// NewCursorRateLimiter creates a new cursor rate limiter
func NewCursorRateLimiter() *CursorRateLimiter {
	return &CursorRateLimiter{
		lastUpdateTime:     make(map[string]time.Time),
		maxUpdatesPerSecond: 20, // Allow 20 cursor updates per second
	}
}

// Allow checks if a cursor update is allowed
func (crl *CursorRateLimiter) Allow(connID string) bool {
	now := time.Now()

	crl.mu.Lock()
	defer crl.mu.Unlock()

	if lastTime, exists := crl.lastUpdateTime[connID]; exists {
		elapsed := now.Sub(lastTime)
		minInterval := time.Second / time.Duration(crl.maxUpdatesPerSecond)
		if elapsed < minInterval {
			return false
		}
	}

	crl.lastUpdateTime[connID] = now
	return true
}
