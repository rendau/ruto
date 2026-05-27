package service

import "testing"

func TestHashPasswordAndComparePassword(t *testing.T) {
	t.Run("hash and compare success", func(t *testing.T) {
		password := "S3cure_Pass!"

		hash, err := hashPassword(password)
		if err != nil {
			t.Fatalf("hashPassword() error = %v", err)
		}
		if hash == "" {
			t.Fatalf("hashPassword() returned empty hash")
		}
		if hash == password {
			t.Fatalf("hashPassword() returned plaintext password")
		}

		ok, err := comparePassword(hash, password)
		if err != nil {
			t.Fatalf("comparePassword() error = %v", err)
		}
		if !ok {
			t.Fatalf("comparePassword() = false, want true")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		hash, err := hashPassword("right-password")
		if err != nil {
			t.Fatalf("hashPassword() error = %v", err)
		}

		ok, err := comparePassword(hash, "wrong-password")
		if err != nil {
			t.Fatalf("comparePassword() error = %v", err)
		}
		if ok {
			t.Fatalf("comparePassword() = true, want false")
		}
	})

	t.Run("too short hash", func(t *testing.T) {
		ok, err := comparePassword("short", "any-password")
		if err != nil {
			t.Fatalf("comparePassword() error = %v", err)
		}
		if ok {
			t.Fatalf("comparePassword() = true, want false")
		}
	})
}
