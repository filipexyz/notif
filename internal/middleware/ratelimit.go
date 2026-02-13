package middleware

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	// DefaultRatePerSecond is the default rate limit for authenticated requests
	DefaultRatePerSecond int
	// DefaultBurst is the default burst size (max requests in a burst)
	DefaultBurst int
	// UnauthRatePerSecond is the rate limit for unauthenticated requests (by IP)
	UnauthRatePerSecond int
	// UnauthBurst is the burst size for unauthenticated requests
	UnauthBurst int
	// CleanupInterval is how often to clean up old limiters
	CleanupInterval time.Duration
	// MaxAge is how long to keep a limiter after last use
	MaxAge time.Duration
}

// DefaultRateLimitConfig returns sensible defaults
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		DefaultRatePerSecond: 100,
		DefaultBurst:         200,
		UnauthRatePerSecond:  10,
		UnauthBurst:          20,
		CleanupInterval:      5 * time.Minute,
		MaxAge:               10 * time.Minute,
	}
}

// rateLimiterEntry holds a limiter and its last access time
type rateLimiterEntry struct {
	limiter      *rate.Limiter
	lastSeenNano atomic.Int64
}

// RateLimiter manages per-key rate limiters
type RateLimiter struct {
	config   RateLimitConfig
	limiters sync.Map // map[string]*rateLimiterEntry
	stopCh   chan struct{}
}

// NewRateLimiter creates a new rate limiter with the given config
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		config: config,
		stopCh: make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// cleanup periodically removes old limiters
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			rl.limiters.Range(func(key, value interface{}) bool {
				entry := value.(*rateLimiterEntry)
				lastSeen := time.Unix(0, entry.lastSeenNano.Load())
				if now.Sub(lastSeen) > rl.config.MaxAge {
					rl.limiters.Delete(key)
				}
				return true
			})
		case <-rl.stopCh:
			return
		}
	}
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// getLimiter returns or creates a limiter for the given key
func (rl *RateLimiter) getLimiter(key string, ratePerSecond, burst int) *rate.Limiter {
	now := time.Now().UnixNano()

	if val, ok := rl.limiters.Load(key); ok {
		entry := val.(*rateLimiterEntry)
		entry.lastSeenNano.Store(now)
		return entry.limiter
	}

	limiter := rate.NewLimiter(rate.Limit(ratePerSecond), burst)
	entry := &rateLimiterEntry{
		limiter: limiter,
	}
	entry.lastSeenNano.Store(now)
	actual, _ := rl.limiters.LoadOrStore(key, entry)
	return actual.(*rateLimiterEntry).limiter
}

// Allow checks if a request is allowed for the given key and rate
func (rl *RateLimiter) Allow(key string, ratePerSecond, burst int) bool {
	limiter := rl.getLimiter(key, ratePerSecond, burst)
	return limiter.Allow()
}

// RateLimit creates middleware that enforces rate limits
// Uses API key rate limit if available, falls back to defaults
func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var key string
			var ratePerSecond, burst int

			// Check if we have an authenticated context with API key info
			authCtx := GetAuthContext(r.Context())

			if authCtx != nil && authCtx.APIKeyID != nil {
				// Use API key ID as the rate limit key
				key = "apikey:" + authCtx.APIKeyID.String()

				// Get rate limit from context (set by auth middleware)
				if customRate := GetRateLimit(r.Context()); customRate > 0 {
					ratePerSecond = customRate
					burst = customRate * 2 // Allow burst of 2x the rate
				} else {
					ratePerSecond = rl.config.DefaultRatePerSecond
					burst = rl.config.DefaultBurst
				}
			} else if authCtx != nil && authCtx.UserID != nil {
				// Clerk user - use user ID as key
				key = "user:" + *authCtx.UserID
				ratePerSecond = rl.config.DefaultRatePerSecond
				burst = rl.config.DefaultBurst
			} else {
				// Unauthenticated - use IP as key with stricter limits
				// Extract IP without port
				ip := r.RemoteAddr
				if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
					ip = host
				}
				key = "ip:" + ip
				ratePerSecond = rl.config.UnauthRatePerSecond
				burst = rl.config.UnauthBurst
			}

			if !rl.Allow(key, ratePerSecond, burst) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(ratePerSecond))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Context key for rate limit
type rateLimitKey struct{}

// SetRateLimit stores the rate limit in the request context
func SetRateLimit(ctx context.Context, ratePerSecond int) context.Context {
	return context.WithValue(ctx, rateLimitKey{}, ratePerSecond)
}

// GetRateLimit retrieves the rate limit from context
func GetRateLimit(ctx context.Context) int {
	if v := ctx.Value(rateLimitKey{}); v != nil {
		return v.(int)
	}
	return 0
}
