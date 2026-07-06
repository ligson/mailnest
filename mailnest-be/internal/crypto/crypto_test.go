package crypto

import "testing"

func TestEncryptDecrypt(t *testing.T) {
	secret := "change-me-32-byte-secret-value"
	plaintext := "mail-password"

	ciphertext, err := EncryptString(plaintext, secret)
	if err != nil {
		t.Fatalf("EncryptString returned error: %v", err)
	}
	if ciphertext == plaintext {
		t.Fatal("expected encrypted value to differ from plaintext")
	}

	decrypted, err := DecryptString(ciphertext, secret)
	if err != nil {
		t.Fatalf("DecryptString returned error: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("expected decrypted plaintext %q, got %q", plaintext, decrypted)
	}
}
