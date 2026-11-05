package token

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	raw, err := Sign("secret", Claims{UserID: 42, Email: "a@b.com"}, time.Hour)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	got, err := Verify("secret", raw)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if got.UserID != 42 || got.Email != "a@b.com" {
		t.Errorf("Verify() = %+v, want UserID 42 and Email a@b.com", got)
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	raw, err := Sign("secret", Claims{UserID: 1}, time.Hour)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	if _, err := Verify("other-secret", raw); err == nil {
		t.Fatal("Verify() accepted a token signed with another secret")
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	raw, err := Sign("secret", Claims{UserID: 1}, -time.Minute)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	if _, err := Verify("secret", raw); err == nil {
		t.Fatal("Verify() accepted an expired token")
	}
}

func TestVerifyRejectsAlgNone(t *testing.T) {
	enc := base64.RawURLEncoding
	payload := enc.EncodeToString([]byte(`{"sub":"1","exp":9999999999}`))
	raw := enc.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + payload + "."

	if _, err := Verify("secret", raw); err == nil {
		t.Fatal("Verify() accepted an alg=none token")
	}
}
