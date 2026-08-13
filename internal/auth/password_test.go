package auth

import "testing"

func TestHashAndVerifyPassword_RoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if err := VerifyPassword(hash, "correct horse battery staple"); err != nil {
		t.Errorf("VerifyPassword with correct password: %v", err)
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if err := VerifyPassword(hash, "wrong password"); err != ErrPasswordMismatch {
		t.Errorf("VerifyPassword with wrong password: got %v, want ErrPasswordMismatch", err)
	}
}

func TestHashPassword_UniqueSaltPerCall(t *testing.T) {
	hashA, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	hashB, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if hashA == hashB {
		t.Error("two hashes of the same password should differ due to random salts")
	}

	if err := VerifyPassword(hashB, "same password"); err != nil {
		t.Errorf("VerifyPassword(hashB): %v", err)
	}
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	cases := []string{
		"",
		"not a hash at all",
		"$argon2id$v=19$m=65536,t=3,p=4$onlyonefield",
		"$bcrypt$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA",
	}
	for _, c := range cases {
		if err := VerifyPassword(c, "anything"); err == nil {
			t.Errorf("VerifyPassword(%q): expected error, got nil", c)
		}
	}
}
