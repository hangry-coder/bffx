package admin

import (
	"testing"
	"time"
)

func TestSessionPayloadExpired(t *testing.T) {
	future := time.Now().Add(time.Hour).Unix()
	if sessionPayloadExpired(future, 0) {
		t.Fatal("future absolute expiry should be valid")
	}
	past := time.Now().Add(-time.Hour).Unix()
	if !sessionPayloadExpired(past, 0) {
		t.Fatal("past absolute expiry should be expired")
	}
}

func TestParseSessionPayload_LegacyEmail(t *testing.T) {
	email := "admin@example.com"
	signed := signValue(email)
	got, _, _, _, ok := parseSessionPayload(signed)
	if !ok || got != email {
		t.Fatalf("legacy parse = %q ok=%v", got, ok)
	}
}

func TestParseSessionPayload_V2(t *testing.T) {
	issued := time.Now().Unix()
	expires := time.Now().Add(time.Hour).Unix()
	payload := formatSessionPayload("admin@example.com", issued, expires, 0)
	signed := signValue(payload)
	email, gotIssued, gotExpires, _, ok := parseSessionPayload(signed)
	if !ok || email != "admin@example.com" || gotIssued != issued || gotExpires != expires {
		t.Fatalf("v2 parse failed: %q %d %d ok=%v", email, gotIssued, gotExpires, ok)
	}
}
