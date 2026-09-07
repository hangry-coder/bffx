package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// VerifyStripeSignature checks the Stripe-Signature header against the raw webhook
// body using the endpoint signing secret (whsec_...).
func VerifyStripeSignature(payload []byte, sigHeader, secret string, tolerance time.Duration) error {
	if secret == "" || sigHeader == "" {
		return fmt.Errorf("stripe webhook: missing secret or signature header")
	}
	var tsStr string
	var v1Sigs []string
	for _, part := range strings.Split(sigHeader, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "t=") {
			tsStr = strings.TrimPrefix(part, "t=")
		}
		if strings.HasPrefix(part, "v1=") {
			v1Sigs = append(v1Sigs, strings.TrimPrefix(part, "v1="))
		}
	}
	if tsStr == "" || len(v1Sigs) == 0 {
		return fmt.Errorf("stripe webhook: malformed Stripe-Signature header")
	}
	tsInt, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return fmt.Errorf("stripe webhook: bad timestamp: %w", err)
	}
	ts := time.Unix(tsInt, 0)
	now := time.Now()
	if ts.Before(now.Add(-tolerance)) || ts.After(now.Add(tolerance)) {
		return fmt.Errorf("stripe webhook: timestamp outside tolerance")
	}

	signed := tsStr + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signed))
	expected := mac.Sum(nil)

	for _, sigHex := range v1Sigs {
		sigBytes, err := hex.DecodeString(sigHex)
		if err != nil {
			continue
		}
		if len(sigBytes) == len(expected) && subtle.ConstantTimeCompare(sigBytes, expected) == 1 {
			return nil
		}
	}
	return fmt.Errorf("stripe webhook: signature mismatch")
}
