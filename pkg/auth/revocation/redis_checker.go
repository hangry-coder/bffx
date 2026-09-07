package revocation

import (
	"context"
	"os"
	"time"

	"github.com/hangry-coder/bffx/pkg/logger"

	"github.com/redis/go-redis/v9"
)

type Checker interface {
	IsRevoked(jti string) bool
	Revoke(ctx context.Context, jti string, ttl time.Duration) error
}

// RedisChecker implements the JWT revocation check against a Redis backend.
//
// Failure mode is environment-aware so we don't lock everyone out on a Redis
// blip in production *or* silently let revoked tokens back in:
//
//   * BFFX_ENV=production → fail-CLOSED. A Redis error is treated as
//     "revoked", so a connectivity issue degrades to "users have to log back
//     in" rather than "we can't enforce logout/revocation any more".
//   * Anywhere else (dev, test, staging without the env set) → fail-OPEN.
//
// Override either way explicitly via BFFX_JWT_REVOCATION_FAIL_CLOSED=1|0.
type RedisChecker struct {
	rdb        *redis.Client
	prefix     string
	failClosed bool
}

func NewRedisChecker(rdb *redis.Client) *RedisChecker {
	return &RedisChecker{
		rdb:        rdb,
		prefix:     "bffx:jwt:revoked:",
		failClosed: defaultFailClosed(),
	}
}

func defaultFailClosed() bool {
	switch os.Getenv("BFFX_JWT_REVOCATION_FAIL_CLOSED") {
	case "1", "true", "TRUE", "yes":
		return true
	case "0", "false", "FALSE", "no":
		return false
	}
	return os.Getenv("BFFX_ENV") == "production"
}

// SetFailClosed lets callers override the default policy programmatically
// (mostly for tests).
func (c *RedisChecker) SetFailClosed(b bool) { c.failClosed = b }

// IsRevoked reports whether the JTI is on the denylist. See RedisChecker doc
// comment for the failure-mode policy.
func (c *RedisChecker) IsRevoked(jti string) bool {
	if c.rdb == nil || jti == "" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	val, err := c.rdb.Exists(ctx, c.prefix+jti).Result()
	if err != nil {
		logger.Warn("jwt revocation check failed: %v (fail_closed=%v)", err, c.failClosed)
		return c.failClosed
	}
	return val > 0
}

// Revoke adds a JTI to the denylist with a specific TTL.
func (c *RedisChecker) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	if c.rdb == nil || jti == "" {
		return nil
	}
	if ttl <= 0 {
		return nil
	}

	return c.rdb.Set(ctx, c.prefix+jti, "1", ttl).Err()
}
