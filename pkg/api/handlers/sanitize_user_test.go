package handlers

import (
	"testing"
)

func TestSanitizeUser_DropsSensitiveFields(t *testing.T) {
	row := map[string]any{
		"id":            "u1",
		"email":         "alice@example.com",
		"name":          "Alice",
		"password":      "$2a$12$hashedpw",
		"password_hash": "$2a$12$hashedpw",
		"otp_code":      "123456",
		"otp_expiry":    "2026-01-01T00:00:00Z",
		"refresh_token": "rt-abc",
		"jwt_secret":    "do-not-leak",
		"role":          "user",
		"created_at":    "2026-01-01T00:00:00Z",
	}

	out := SanitizeUser(row)

	for _, leaked := range []string{
		"password", "password_hash", "otp_code", "otp_expiry",
		"refresh_token", "jwt_secret",
	} {
		if _, ok := out[leaked]; ok {
			t.Errorf("SanitizeUser leaked %q", leaked)
		}
	}
	for _, kept := range []string{"id", "email", "name", "role", "created_at"} {
		if _, ok := out[kept]; !ok {
			t.Errorf("SanitizeUser dropped public field %q", kept)
		}
	}
}

// TestSanitizeUser_NewFieldNotLeaked is the regression test the audit asked
// for: any *new* field appearing on a User row must NOT be emitted by
// SanitizeUser unless someone has explicitly added it to PublicUserFields.
//
// If you intentionally added a field to PublicUserFields, you also need to
// update this test (delete the failing field name from the synthetic row or
// pick a different sentinel name). That's the whole point — additions are
// reviewed, not silent.
func TestSanitizeUser_NewFieldNotLeaked(t *testing.T) {
	row := map[string]any{
		"id":                            "u1",
		"email":                         "alice@example.com",
		"hypothetical_password_hint":    "the dog's name",
		"hypothetical_2fa_seed":         "JBSWY3DPEHPK3PXP",
		"hypothetical_internal_score":   42,
		"hypothetical_admin_note":       "VIP, do not delete",
	}
	out := SanitizeUser(row)
	for _, name := range []string{
		"hypothetical_password_hint",
		"hypothetical_2fa_seed",
		"hypothetical_internal_score",
		"hypothetical_admin_note",
	} {
		if _, ok := out[name]; ok {
			t.Errorf("SanitizeUser leaked unknown field %q — adding new User fields must NOT default to public; add to PublicUserFields explicitly if intended", name)
		}
	}
}

func TestPublicUserFields_NoSecretsAccidentallyOnAllowlist(t *testing.T) {
	banned := map[string]bool{
		"password":      true,
		"password_hash": true,
		"otp_code":      true,
		"otp_expiry":    true,
		"refresh_token": true,
		"jwt_secret":    true,
		"api_key":       true,
		"secret":        true,
	}
	for _, f := range PublicUserFields {
		if banned[f] {
			t.Errorf("PublicUserFields includes sensitive field %q — must not be exposed", f)
		}
	}
}
