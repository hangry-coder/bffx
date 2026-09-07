package metrics

import (
	"sync/atomic"
)

type Metrics struct {
	TotalRequests atomic.Int64
	Requests4xx   atomic.Int64
	Requests5xx   atomic.Int64

	AnonymousAuthAttempts    atomic.Int64
	AnonymousAuthRateLimited atomic.Int64
	AnonymousAuthBlocked     atomic.Int64
}

var Global = &Metrics{}
