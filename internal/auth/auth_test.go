package auth

import (
	"testing"
	"time"
)

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("super-secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !CheckPassword(hash, "super-secret") {
		t.Fatal("expected password to match")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("expected wrong password to fail")
	}
}

func TestShortPasswordRejected(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected short password error")
	}
}

func TestIssueAndParseToken(t *testing.T) {
	token, err := IssueToken("secret", time.Minute, 42, "dev@example.com")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	claims, err := ParseToken("secret", token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.UserID != 42 || claims.Email != "dev@example.com" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}
