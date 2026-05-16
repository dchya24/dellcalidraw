package middleware

import (
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// IPRateLimiter provides per-IP rate limiting using token bucket algorithm.
type IPRateLimiter struct {
	limiters sync.Map
	rate     rate.Limit
	burst    int
}

// NewIPRateLimiter creates a new per-IP rate limiter.
// r = requests per second, burst = max burst size.
func NewIPRateLimiter(r float64, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		rate:  rate.Limit(r),
		burst: burst,
	}
}

func (l *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	if entry, ok := l.limiters.Load(ip); ok {
		return entry.(*rate.Limiter)
	}

	limiter := rate.NewLimiter(l.rate, l.burst)
	l.limiters.Store(ip, limiter)
	return limiter
}

// Middleware returns an HTTP middleware that rate limits by IP.
func (l *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		limiter := l.getLimiter(ip)

		if !limiter.Allow() {
			http.Error(w, `{"error":"rate limit exceeded","message":"Too many requests. Please try again later."}`, http.StatusTooManyRequests)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "10")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractIP gets the real client IP from request headers or RemoteAddr.
func extractIP(r *http.Request) string {
	// Check X-Forwarded-For header (from reverse proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		return xff[:min(len(xff), 45)]
	}
	// Check X-Real-Ip header
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return xri[:min(len(xri), 45)]
	}
	// Fallback to RemoteAddr
	addr := r.RemoteAddr
	if len(addr) > 45 {
		addr = addr[:45]
	}
	return addr
}
