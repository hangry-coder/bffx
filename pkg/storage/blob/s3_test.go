package blob

import (
	"errors"
	"testing"
)

func TestNewS3ProviderFromEnv_MissingVars(t *testing.T) {
	t.Setenv("BFFX_S3_ENDPOINT", "")
	t.Setenv("BFFX_S3_ACCESS_KEY", "")
	t.Setenv("BFFX_S3_SECRET_KEY", "")
	t.Setenv("BFFX_S3_BUCKET", "")
	_, err := NewS3ProviderFromEnv()
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestS3Provider_InvalidKey(t *testing.T) {
	// Client is nil; we only exercise ValidateObjectKey path if we called PresignPut —
	// instead test ValidateObjectKey directly used by S3:
	if err := ValidateObjectKey("../x"); err == nil {
		t.Fatal("expected error")
	}
}
