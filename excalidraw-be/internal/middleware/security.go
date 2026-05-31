package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeadersConfig configures HTTP security headers middleware.
//
// Defaults are tuned for an SPA + WebSocket backend behind a TLS
// terminating reverse proxy. HSTS is opt-in (EnableHSTS) because
// returning it over plain HTTP poisons the browser's HSTS cache for
// the host and can lock dev environments out.
type SecurityHeadersConfig struct {
	// EnableHSTS sets Strict-Transport-Security. Set true ONLY in
	// production where TLS is guaranteed. max-age 1 year, no
	// preload (opt in via SetHSTSPreload below).
	EnableHSTS bool

	// HSTSMaxAge in seconds; defaults to 31536000 (1 year) when zero.
	HSTSMaxAge int

	// SetHSTSPreload adds the `preload` directive. Only enable after
	// vetting at https://hstspreload.org/.
	SetHSTSPreload bool

	// CSP (Content-Security-Policy). Empty disables. The default is
	// permissive enough for the Excalidraw SPA + collab WS but blocks
	// arbitrary script injection via inline.
	CSP string

	// FrameOptions: typical values "DENY" or "SAMEORIGIN". Empty disables.
	FrameOptions string

	// ReferrerPolicy: e.g. "strict-origin-when-cross-origin". Empty disables.
	ReferrerPolicy string

	// PermissionsPolicy: e.g. "camera=(), microphone=(), geolocation=()".
	PermissionsPolicy string

	// CrossOriginOpenerPolicy: e.g. "same-origin". Empty disables.
	CrossOriginOpenerPolicy string
}

// DefaultSecurityHeaders returns a sensible production-friendly config.
// Caller MUST flip EnableHSTS=true once the deployment is on HTTPS.
func DefaultSecurityHeaders() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		EnableHSTS:     false,
		HSTSMaxAge:     31536000,
		SetHSTSPreload: false,

		// Permits the SPA bundle, fonts, images, and outbound calls
		// to the same origin + websockets. `unsafe-inline` is
		// included for styles because Excalidraw injects style attrs;
		// scripts stay strict.
		CSP: strings.Join([]string{
			"default-src 'self'",
			"script-src 'self'",
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' data: blob:",
			"font-src 'self' data:",
			"connect-src 'self' ws: wss:",
			"frame-ancestors 'none'",
			"base-uri 'self'",
			"form-action 'self'",
		}, "; "),

		FrameOptions:            "DENY",
		ReferrerPolicy:          "strict-origin-when-cross-origin",
		PermissionsPolicy:       "camera=(), microphone=(), geolocation=(), payment=(), usb=()",
		CrossOriginOpenerPolicy: "same-origin",
	}
}

// SecurityHeaders returns an HTTP middleware that sets the configured
// security response headers on every response. Headers are set BEFORE
// the handler runs so they survive even if the handler short-circuits.
func SecurityHeaders(cfg SecurityHeadersConfig) func(http.Handler) http.Handler {
	hstsValue := ""
	if cfg.EnableHSTS {
		maxAge := cfg.HSTSMaxAge
		if maxAge == 0 {
			maxAge = 31536000
		}
		hstsValue = "max-age=" + itoa(maxAge) + "; includeSubDomains"
		if cfg.SetHSTSPreload {
			hstsValue += "; preload"
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()

			// Always-on minimal headers
			h.Set("X-Content-Type-Options", "nosniff")

			if hstsValue != "" {
				h.Set("Strict-Transport-Security", hstsValue)
			}
			if cfg.CSP != "" {
				h.Set("Content-Security-Policy", cfg.CSP)
			}
			if cfg.FrameOptions != "" {
				h.Set("X-Frame-Options", cfg.FrameOptions)
			}
			if cfg.ReferrerPolicy != "" {
				h.Set("Referrer-Policy", cfg.ReferrerPolicy)
			}
			if cfg.PermissionsPolicy != "" {
				h.Set("Permissions-Policy", cfg.PermissionsPolicy)
			}
			if cfg.CrossOriginOpenerPolicy != "" {
				h.Set("Cross-Origin-Opener-Policy", cfg.CrossOriginOpenerPolicy)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// itoa avoids pulling strconv just for one int conversion.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
