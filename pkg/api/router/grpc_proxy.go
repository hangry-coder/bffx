package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"google.golang.org/grpc/metadata"
)

// NewGRPCProxyHTTPRequest builds an HTTP request that replays a gRPC unary call
// against the REST router. For GET routes, [payloadJSON] fields are merged into
// the query string; for other methods they become the JSON body. Incoming gRPC
// metadata is copied to HTTP headers, and BFFX_APP_SECRET is injected when the
// client did not send X-App-Secret (internal hop after gRPC auth).
func NewGRPCProxyHTTPRequest(ctx context.Context, method, path string, payloadJSON []byte) (*http.Request, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodPost
	}

	var body io.Reader
	targetPath := path

	if len(payloadJSON) > 0 {
		if method == http.MethodGet || method == http.MethodHead {
			var payload map[string]any
			if err := json.Unmarshal(payloadJSON, &payload); err != nil {
				return nil, err
			}
			q, err := url.Parse(targetPath)
			if err != nil {
				return nil, err
			}
			vals := q.Query()
			for k, v := range payload {
				if v == nil {
					continue
				}
				vals.Set(k, fmt.Sprintf("%v", v))
			}
			q.RawQuery = vals.Encode()
			targetPath = q.String()
		} else {
			body = bytes.NewReader(payloadJSON)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, targetPath, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		for k, vals := range md {
			for _, v := range vals {
				req.Header.Add(k, v)
			}
		}
	}

	if secret := strings.TrimSpace(os.Getenv("BFFX_APP_SECRET")); secret != "" {
		if strings.TrimSpace(req.Header.Get("X-App-Secret")) == "" {
			req.Header.Set("X-App-Secret", secret)
		}
	}

	return req, nil
}
