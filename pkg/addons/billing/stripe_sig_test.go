package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func TestVerifyStripeSignature_Accept(t *testing.T) {
	secret := "whsec_testsecret"
	body := []byte(`{"id":"evt_1","object":"event"}`)
	ts := time.Now().Unix()
	signed := fmt.Sprintf("%d.%s", ts, string(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signed))
	sig := hex.EncodeToString(mac.Sum(nil))
	hdr := fmt.Sprintf("t=%d,v1=%s", ts, sig)
	if err := VerifyStripeSignature(body, hdr, secret, 300*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyStripeSignature_RejectWrongSecret(t *testing.T) {
	body := []byte(`{}`)
	ts := time.Now().Unix()
	signed := fmt.Sprintf("%d.%s", ts, string(body))
	mac := hmac.New(sha256.New, []byte("whsec_a"))
	mac.Write([]byte(signed))
	sig := hex.EncodeToString(mac.Sum(nil))
	hdr := fmt.Sprintf("t=%d,v1=%s", ts, sig)
	if err := VerifyStripeSignature(body, hdr, "whsec_b", 300*time.Second); err == nil {
		t.Fatal("expected error")
	}
}
