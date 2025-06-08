package github

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/KOFI-GYIMAH/github-monitor/pkg/logger"
)

type RateLimiter struct {
	mu          sync.Mutex
	remaining   int
	reset       time.Time
	lowWarn     int
	retryAfter  time.Duration
	retryStatus int
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		remaining:   5000,
		reset:       time.Now(),
		lowWarn:     100,
		retryStatus: http.StatusTooManyRequests,
	}
}

func (r *RateLimiter) waitIfNeeded() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if r.remaining <= 0 && now.Before(r.reset) {
		waitTime := time.Until(r.reset)
		logger.Warn("[RateLimiter] Rate limit exceeded. Waiting %v until reset at %v", waitTime, r.reset)
		time.Sleep(waitTime)
	}
}

func (r *RateLimiter) updateFromHeaders(headers http.Header) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if remaining := headers.Get("X-RateLimit-Remaining"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			r.remaining = val
		}
	}

	if reset := headers.Get("X-RateLimit-Reset"); reset != "" {
		if val, err := strconv.ParseInt(reset, 10, 64); err == nil {
			r.reset = time.Unix(val, 0)
		}
	}

	if retry := headers.Get("Retry-After"); retry != "" {
		if seconds, err := strconv.Atoi(retry); err == nil {
			r.retryAfter = time.Duration(seconds) * time.Second
		}
	}

	if r.remaining < r.lowWarn {
		logger.Warn("[RateLimiter] Low rate limit: %d remaining. Resets at %s", r.remaining, r.reset.Format(time.RFC1123))
	}
}

func (r *RateLimiter) Middleware(next http.RoundTripper) http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		r.waitIfNeeded()

		resp, err := next.RoundTrip(req)
		if err != nil {
			logger.Error("Network error in RoundTrip: %v", err)
			return nil, err
		}

		// * Retry on 429
		if resp.StatusCode == r.retryStatus {
			r.updateFromHeaders(resp.Header)
			logger.Warn("[RateLimiter] Received 429. Retrying after %v...", r.retryAfter)
			time.Sleep(r.retryAfter)
			return next.RoundTrip(req)
		}

		r.updateFromHeaders(resp.Header)
		return resp, nil
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
