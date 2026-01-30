package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRateLimiter_BasicLimit(t *testing.T) {
	config := RateLimitConfig{
		DefaultRatePerSecond: 10,
		DefaultBurst:         10,
		UnauthRatePerSecond:  5,
		UnauthBurst:          5,
		CleanupInterval:      time.Minute,
		MaxAge:               time.Minute,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Should allow up to burst size
	for i := 0; i < 10; i++ {
		if !rl.Allow("test-key", 10, 10) {
			t.Errorf("Request %d should have been allowed", i)
		}
	}

	// Next request should be denied
	if rl.Allow("test-key", 10, 10) {
		t.Error("Request should have been rate limited")
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	config := RateLimitConfig{
		DefaultRatePerSecond: 5,
		DefaultBurst:         5,
		UnauthRatePerSecond:  5,
		UnauthBurst:          5,
		CleanupInterval:      time.Minute,
		MaxAge:               time.Minute,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Exhaust key1
	for i := 0; i < 5; i++ {
		rl.Allow("key1", 5, 5)
	}

	// key1 should be limited
	if rl.Allow("key1", 5, 5) {
		t.Error("key1 should be rate limited")
	}

	// key2 should still work
	if !rl.Allow("key2", 5, 5) {
		t.Error("key2 should not be rate limited")
	}
}

func TestRateLimitMiddleware_WithAPIKey(t *testing.T) {
	config := RateLimitConfig{
		DefaultRatePerSecond: 5,
		DefaultBurst:         5,
		UnauthRatePerSecond:  2,
		UnauthBurst:          2,
		CleanupInterval:      time.Minute,
		MaxAge:               time.Minute,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	keyID := uuid.New()
	authCtx := &AuthContext{
		OrgID:    "test-org",
		APIKeyID: &keyID,
	}

	// Make requests with API key auth
	allowed := 0
	limited := 0

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(setAuthContext(req.Context(), authCtx))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			allowed++
		} else if w.Code == http.StatusTooManyRequests {
			limited++
		}
	}

	if allowed != 5 {
		t.Errorf("Expected 5 allowed requests, got %d", allowed)
	}
	if limited != 5 {
		t.Errorf("Expected 5 limited requests, got %d", limited)
	}
}

func TestRateLimitMiddleware_UnauthenticatedByIP(t *testing.T) {
	config := RateLimitConfig{
		DefaultRatePerSecond: 100,
		DefaultBurst:         100,
		UnauthRatePerSecond:  3,
		UnauthBurst:          3,
		CleanupInterval:      time.Minute,
		MaxAge:               time.Minute,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make unauthenticated requests (stricter limit)
	allowed := 0
	limited := 0

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			allowed++
		} else if w.Code == http.StatusTooManyRequests {
			limited++
		}
	}

	if allowed != 3 {
		t.Errorf("Expected 3 allowed requests for unauthenticated, got %d", allowed)
	}
	if limited != 7 {
		t.Errorf("Expected 7 limited requests, got %d", limited)
	}
}

func TestRateLimitMiddleware_CustomRateFromContext(t *testing.T) {
	config := RateLimitConfig{
		DefaultRatePerSecond: 5,
		DefaultBurst:         5,
		UnauthRatePerSecond:  2,
		UnauthBurst:          2,
		CleanupInterval:      time.Minute,
		MaxAge:               time.Minute,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	keyID := uuid.New()
	authCtx := &AuthContext{
		OrgID:    "test-org",
		APIKeyID: &keyID,
	}

	// Make requests with custom rate limit (10/s)
	allowed := 0

	for i := 0; i < 25; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := setAuthContext(req.Context(), authCtx)
		ctx = SetRateLimit(ctx, 20) // Custom rate of 20/s, burst of 40
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			allowed++
		}
	}

	// Should allow 40 requests (burst = rate * 2 = 20 * 2 = 40)
	if allowed < 20 {
		t.Errorf("Expected at least 20 allowed requests with custom rate, got %d", allowed)
	}
}

func TestRateLimitMiddleware_ResponseHeaders(t *testing.T) {
	config := RateLimitConfig{
		DefaultRatePerSecond: 1,
		DefaultBurst:         1,
		UnauthRatePerSecond:  1,
		UnauthBurst:          1,
		CleanupInterval:      time.Minute,
		MaxAge:               time.Minute,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request - allowed
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Error("First request should be allowed")
	}

	// Second request - limited
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Error("Second request should be rate limited")
	}

	// Check headers
	if w.Header().Get("Retry-After") != "1" {
		t.Error("Expected Retry-After header")
	}
	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("Expected X-RateLimit-Limit header")
	}
	if w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Error("Expected X-RateLimit-Remaining: 0")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	config := RateLimitConfig{
		DefaultRatePerSecond: 100,
		DefaultBurst:         100,
		UnauthRatePerSecond:  100,
		UnauthBurst:          100,
		CleanupInterval:      time.Minute,
		MaxAge:               time.Minute,
	}

	rl := NewRateLimiter(config)
	defer rl.Stop()

	var allowed int64
	var wg sync.WaitGroup

	// 10 goroutines, each making 20 requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if rl.Allow("concurrent-key", 100, 100) {
					atomic.AddInt64(&allowed, 1)
				}
			}
		}()
	}

	wg.Wait()

	// Should allow exactly 100 (burst size)
	if allowed != 100 {
		t.Errorf("Expected exactly 100 allowed requests, got %d", allowed)
	}
}

// Helper to set auth context
func setAuthContext(ctx interface{ Value(any) any }, authCtx *AuthContext) interface {
	Value(any) any
	Done() <-chan struct{}
	Err() error
	Deadline() (time.Time, bool)
} {
	return &testContext{parent: ctx, authCtx: authCtx}
}

type testContext struct {
	parent  interface{ Value(any) any }
	authCtx *AuthContext
}

func (c *testContext) Value(key any) any {
	if key == authCtxKey {
		return c.authCtx
	}
	return c.parent.Value(key)
}
func (c *testContext) Done() <-chan struct{}       { return nil }
func (c *testContext) Err() error                  { return nil }
func (c *testContext) Deadline() (time.Time, bool) { return time.Time{}, false }
