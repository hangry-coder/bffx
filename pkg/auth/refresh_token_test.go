package auth

import "testing"

func TestMintAndParseRefreshToken(t *testing.T) {
	rowID, wire, hash, err := MintRefreshTokenCredential()
	if err != nil {
		t.Fatal(err)
	}
	if rowID == "" || wire == "" || hash == "" {
		t.Fatal("expected credential parts")
	}
	id, secret, ok := ParseRefreshWire(wire)
	if !ok || id != rowID {
		t.Fatalf("parse wire: id=%s secret=%s ok=%v", id, secret, ok)
	}
	if !VerifyRefreshSecret(hash, secret) {
		t.Fatal("hash verify failed")
	}
}
