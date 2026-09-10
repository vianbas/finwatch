package auth

import "testing"

func TestHashPassword_VerifyRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct-password" {
		t.Fatalf("hash must not equal the plaintext")
	}
	if err := VerifyPassword(hash, "correct-password"); err != nil {
		t.Errorf("VerifyPassword with correct password: %v", err)
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := VerifyPassword(hash, "wrong-password"); err == nil {
		t.Errorf("VerifyPassword with wrong password: want error, got nil")
	}
}
