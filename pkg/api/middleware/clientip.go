package middleware

import (
	"net"
	"net/http"
	"os"
	"strings"
)

// TrustForwardedHeaders returns true when BFFX_TRUST_X_FORWARDED_FOR is set to "true".
// Only enable behind a trusted reverse proxy that strips spoofed X-Forwarded-For.
func TrustForwardedHeaders() bool {
	return os.Getenv("BFFX_TRUST_X_FORWARDED_FOR") == "true"
}

// ClientIP returns the best-effort client address for rate limiting and auditing.
// When trustForwarded is true, uses X-Forwarded-For (first hop) or X-Real-IP, else RemoteAddr.
func ClientIP(r *http.Request, trustForwarded bool) string {
	if trustForwarded {
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
		if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
			return xr
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
