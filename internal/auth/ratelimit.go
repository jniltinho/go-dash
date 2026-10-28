package auth

import (
	"sync"
	"time"
)

// RateLimiter is a fixed-window limiter keyed by client IP, with an extra
// global limit across all clients.
type RateLimiter struct {
	mu        sync.Mutex
	perKey    int
	global    int
	window    time.Duration
	now       func() time.Time
	start     time.Time
	counts    map[string]int
	globalCnt int
}

// NewRateLimiter allows perKey attempts per key and global attempts in total per window.
func NewRateLimiter(perKey, global int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		perKey: perKey,
		global: global,
		window: window,
		now:    time.Now,
		counts: map[string]int{},
	}
}

// Allow records an attempt for key and reports whether it is within limits.
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	if now.Sub(r.start) >= r.window {
		r.start = now
		r.counts = map[string]int{}
		r.globalCnt = 0
	}

	if r.counts[key] >= r.perKey || r.globalCnt >= r.global {
		return false
	}
	r.counts[key]++
	r.globalCnt++
	return true
}
