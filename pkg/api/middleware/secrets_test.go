package middleware

import "testing"

func TestSecretsEqual(t *testing.T) {
	if !SecretsEqual("same", "same") {
		t.Fatal("expected equal secrets to match")
	}
	if SecretsEqual("a", "b") {
		t.Fatal("expected different secrets to mismatch")
	}
}
