package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"
	"wattwatch/internal/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter implements rate limiting using token bucket algorithm
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	cleanup  time.Duration
	window   int // Store window size for header calculations
	requests int // Store total requests for header calculations
}

// NewRateLimiter creates a new rate limiter middleware
func NewRateLimiter(cfg *config.Config) *RateLimiter {
	// Calculate rate as requests per second
	ratePerSecond := rate.Every(time.Duration(cfg.RateLimit.Window) * time.Second / time.Duration(cfg.RateLimit.Requests))

	limiter := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(ratePerSecond),
		burst:    cfg.RateLimit.Requests, // Use total requests as burst
		cleanup:  time.Hour,
		window:   cfg.RateLimit.Window,
		requests: cfg.RateLimit.Requests,
	}

	// Start cleanup routine
	go limiter.cleanupRoutine()

	return limiter
}

// getLimiter returns a rate limiter for the given key
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double check after acquiring write lock
	limiter, exists = rl.limiters[key]
	if exists {
		return limiter
	}

	// Create new limiter with full capacity
	limiter = rate.NewLimiter(rl.rate, rl.burst)

	// Reserve initial tokens to allow immediate requests
	now := time.Now()
	r := limiter.ReserveN(now, 0) // Reserve 0 tokens to get rate info
	if r.OK() {
		delay := r.Delay()
		if delay > 0 {
			limiter.AllowN(now.Add(-delay), 1) // Pre-fill one token if there would be a delay
		}
	}

	rl.limiters[key] = limiter
	return limiter
}

// cleanupRoutine periodically removes old limiters
func (rl *RateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		// In a production environment, you might want to track last access time
		// and only remove limiters that haven't been used for a while
		rl.limiters = make(map[string]*rate.Limiter)
		rl.mu.Unlock()
	}
}

// Middleware returns a Gin middleware function that implements rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip rate limiting for Swagger documentation
		if c.Request.URL.Path == "/swagger/index.html" ||
			c.Request.URL.Path == "/swagger/doc.json" ||
			c.Request.URL.Path == "/swagger/*any" {
			c.Next()
			return
		}

		key := c.ClientIP()
		limiter := rl.getLimiter(key)

		// Try to reserve a token
		now := time.Now()
		r := limiter.ReserveN(now, 1)
		if !r.OK() {
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.requests))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", now.Add(time.Duration(rl.window)*time.Second).Unix()))
			c.Header("Retry-After", fmt.Sprintf("%d", rl.window))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": fmt.Sprintf("%ds", rl.window),
			})
			c.Abort()
			return
		}

		// Calculate delay
		delay := r.Delay()
		if delay > 0 {
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.requests))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", now.Add(delay).Unix()))
			c.Header("Retry-After", fmt.Sprintf("%d", int(delay.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": fmt.Sprintf("%ds", int(delay.Seconds())),
			})
			c.Abort()
			return
		}

		// Calculate remaining tokens
		tokens := int(limiter.Tokens())
		if tokens > rl.requests {
			tokens = rl.requests
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.requests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", tokens))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", now.Add(time.Duration(rl.window)*time.Second).Unix()))

		c.Next()
	}
}
