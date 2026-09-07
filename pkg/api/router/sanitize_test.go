package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeResourceResponse_UserResource(t *testing.T) {
	row := map[string]any{
		"id":            "user_123",
		"email":         "alice@example.com",
		"password":      "insecurepassword",
		"otp_code":      "123456",
		"secret_notes":  "some sensitive internal info",
		"custom_field":  "this should be dropped because of User allowlist",
	}

	sanitized := SanitizeResourceResponse("User", row)

	assert.Equal(t, "user_123", sanitized["id"])
	assert.Equal(t, "alice@example.com", sanitized["email"])
	assert.Nil(t, sanitized["password"])
	assert.Nil(t, sanitized["otp_code"])
	assert.Nil(t, sanitized["secret_notes"])
	assert.Nil(t, sanitized["custom_field"])
}

func TestSanitizeResourceResponse_GenericResource(t *testing.T) {
	row := map[string]any{
		"id":           "some_id",
		"title":        "Safe Title",
		"secret":       "my-token-secret",
		"token":        "xyz",
		"otp":          "456",
		"normal_field": "keep this",
	}

	sanitized := SanitizeResourceResponse("AppConfig", row)

	assert.Equal(t, "some_id", sanitized["id"])
	assert.Equal(t, "Safe Title", sanitized["title"])
	assert.Equal(t, "keep this", sanitized["normal_field"])
	assert.Nil(t, sanitized["secret"])
	assert.Nil(t, sanitized["token"])
	assert.Nil(t, sanitized["otp"])
}

func TestSanitizeResourceListResponse(t *testing.T) {
	rows := []map[string]any{
		{"id": "1", "title": "A", "secret": "s1"},
		{"id": "2", "title": "B", "secret": "s2"},
	}

	sanitized := SanitizeResourceListResponse("AppConfig", rows)

	assert.Len(t, sanitized, 2)
	assert.Equal(t, "1", sanitized[0]["id"])
	assert.Nil(t, sanitized[0]["secret"])
	assert.Equal(t, "2", sanitized[1]["id"])
	assert.Nil(t, sanitized[1]["secret"])
}
