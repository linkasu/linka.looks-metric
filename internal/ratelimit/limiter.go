package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu      sync.Mutex
	buckets map[string]bucket
}

type bucket struct {
	Count     int
	ExpiresAt time.Time
}

func New() *Limiter {
	return &Limiter{buckets: make(map[string]bucket)}
}

func (l *Limiter) Allow(key string, limit int, window time.Duration) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	item := l.buckets[key]
	if item.ExpiresAt.Before(now) {
		item = bucket{ExpiresAt: now.Add(window)}
	}
	if item.Count >= limit {
		return false
	}
	item.Count++
	l.buckets[key] = item
	return true
}

func (l *Limiter) Sweep() {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, item := range l.buckets {
		if item.ExpiresAt.Before(now) {
			delete(l.buckets, key)
		}
	}
}
