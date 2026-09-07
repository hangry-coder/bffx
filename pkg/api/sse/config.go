package sse

import "net/http"

type config struct {
	allowOrigin  string
	extraHeaders map[string]string
}

// Option configures SSE response headers and writer behavior.
type Option func(*config)

// WithAllowOrigin sets Access-Control-Allow-Origin (omit when not needed).
func WithAllowOrigin(origin string) Option {
	return func(c *config) {
		c.allowOrigin = origin
	}
}

// WithHeader sets an additional response header (e.g. X-Accel-Buffering).
func WithHeader(key, value string) Option {
	return func(c *config) {
		if c.extraHeaders == nil {
			c.extraHeaders = make(map[string]string)
		}
		c.extraHeaders[key] = value
	}
}

func applyConfig(opts []Option) config {
	cfg := config{}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}

func applyHeaders(w http.ResponseWriter, cfg config) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	if cfg.allowOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", cfg.allowOrigin)
	}
	for k, v := range cfg.extraHeaders {
		w.Header().Set(k, v)
	}
}
