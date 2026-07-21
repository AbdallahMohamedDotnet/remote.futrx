package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	password := "correct horse battery staple"
	first, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword second: %v", err)
	}
	if first == second {
		t.Fatal("password hashes reused a salt")
	}
	if strings.Contains(first, password) {
		t.Fatal("password hash contains the plaintext password")
	}
	if ok, err := VerifyPassword(first, password); err != nil || !ok {
		t.Fatalf("VerifyPassword(correct) = (%v, %v)", ok, err)
	}
	if ok, err := VerifyPassword(first, "wrong password"); err != nil || ok {
		t.Fatalf("VerifyPassword(wrong) = (%v, %v)", ok, err)
	}
}

func TestPasswordPolicy(t *testing.T) {
	if _, err := HashPassword("too-short"); err != ErrPasswordTooShort {
		t.Fatalf("short password error = %v", err)
	}
	if _, err := HashPassword(strings.Repeat("x", passwordMaxBytes+1)); err != ErrPasswordTooLong {
		t.Fatalf("long password error = %v", err)
	}
}
