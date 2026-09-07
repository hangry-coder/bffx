package auth

import (
	"testing"
	"time"
)

func TestOTPService_GenerateAndVerify(t *testing.T) {
	service := NewOTPService()

	code, hash, expiry, err := service.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// 1. Verify code length is 6 digits
	if len(code) != 6 {
		t.Errorf("expected 6-digit code, got: %s (len=%d)", code, len(code))
	}

	// 2. Verify hash is non-empty
	if len(hash) == 0 {
		t.Errorf("expected non-empty bcrypt hash")
	}

	// 3. Verify expiry is ~5 minutes in the future
	now := time.Now()
	diff := expiry.Sub(now)
	if diff < 4*time.Minute || diff > 6*time.Minute {
		t.Errorf("expected expiry to be ~5 minutes in future, got diff: %v", diff)
	}

	// 4. Verify successful validation
	if !service.Verify(hash, code) {
		t.Errorf("expected Verify to succeed for matching code and hash")
	}

	// 5. Verify failed validation with incorrect code
	if service.Verify(hash, "000000") && code != "000000" {
		t.Errorf("expected Verify to fail for incorrect code")
	}
}
