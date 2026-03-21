package crypto_test

import (
	"bytes"
	"testing"

	"github.com/bhata/AutoDreamApplier/internal/crypto"
)

// ── NewEncryptor ──────────────────────────────────────────────────────────────

func TestNewEncryptor_ValidKey(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	enc, err := crypto.NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor with 32-byte key: unexpected error: %v", err)
	}
	if enc == nil {
		t.Fatal("NewEncryptor returned nil")
	}
}

func TestNewEncryptor_ShortKey(t *testing.T) {
	t.Parallel()
	cases := []int{0, 1, 16, 31}
	for _, length := range cases {
		length := length
		t.Run("length", func(t *testing.T) {
			t.Parallel()
			key := make([]byte, length)
			enc, err := crypto.NewEncryptor(key)
			if err == nil {
				t.Errorf("NewEncryptor with %d-byte key: expected error, got nil", length)
			}
			if enc != nil {
				t.Errorf("NewEncryptor with %d-byte key: expected nil encryptor", length)
			}
		})
	}
}

func TestNewEncryptor_LongKey(t *testing.T) {
	t.Parallel()
	key := make([]byte, 64) // 64 bytes — too long
	enc, err := crypto.NewEncryptor(key)
	if err == nil {
		t.Fatal("NewEncryptor with 64-byte key: expected error, got nil")
	}
	if enc != nil {
		t.Fatal("NewEncryptor with 64-byte key: expected nil encryptor")
	}
}

// ── Encrypt → Decrypt roundtrip ───────────────────────────────────────────────

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte("k"), 32)
	enc, err := crypto.NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	plaintext := []byte("super secret credential")
	ciphertext, nonce, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := enc.Decrypt(ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("Decrypt result = %q; want %q", got, plaintext)
	}
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte("z"), 32)
	enc, err := crypto.NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	plaintext := []byte("")
	ciphertext, nonce, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}

	got, err := enc.Decrypt(ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("Decrypt empty result = %q; want empty", got)
	}
}

// ── Ciphertext uniqueness (random nonce) ──────────────────────────────────────

func TestEncrypt_DifferentCiphertextsForSamePlaintext(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte("x"), 32)
	enc, err := crypto.NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	plaintext := []byte("same content")

	ct1, _, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	ct2, _, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}

	// AES-GCM uses a random nonce, so ciphertexts should differ
	if bytes.Equal(ct1, ct2) {
		t.Error("expected different ciphertexts for same plaintext (random nonce)")
	}
}

func TestEncrypt_DifferentPlaintextsProduceDifferentCiphertexts(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte("y"), 32)
	enc, err := crypto.NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	ct1, _, err := enc.Encrypt([]byte("plaintext one"))
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	ct2, _, err := enc.Encrypt([]byte("plaintext two"))
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}

	if bytes.Equal(ct1, ct2) {
		t.Error("different plaintexts produced the same ciphertext")
	}
}

// ── Wrong key on Decrypt → error ──────────────────────────────────────────────

func TestDecrypt_WrongKey_ReturnsError(t *testing.T) {
	t.Parallel()
	key1 := bytes.Repeat([]byte("a"), 32)
	key2 := bytes.Repeat([]byte("b"), 32)

	enc1, err := crypto.NewEncryptor(key1)
	if err != nil {
		t.Fatalf("NewEncryptor key1: %v", err)
	}
	enc2, err := crypto.NewEncryptor(key2)
	if err != nil {
		t.Fatalf("NewEncryptor key2: %v", err)
	}

	ciphertext, nonce, err := enc1.Encrypt([]byte("secret data"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = enc2.Decrypt(ciphertext, nonce)
	if err == nil {
		t.Fatal("Decrypt with wrong key: expected error, got nil")
	}
}

// ── Tampered ciphertext → error ───────────────────────────────────────────────

func TestDecrypt_TamperedCiphertext_ReturnsError(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte("t"), 32)
	enc, err := crypto.NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	ciphertext, nonce, err := enc.Encrypt([]byte("original data"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Flip a bit in the ciphertext
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	if len(tampered) > 0 {
		tampered[0] ^= 0xFF
	}

	_, err = enc.Decrypt(tampered, nonce)
	if err == nil {
		t.Fatal("Decrypt tampered ciphertext: expected error, got nil")
	}
}

// ── Large plaintext ───────────────────────────────────────────────────────────

func TestEncryptDecrypt_LargePlaintext(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte("L"), 32)
	enc, err := crypto.NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	plaintext := bytes.Repeat([]byte("ABCDEF"), 1000) // 6 KB
	ciphertext, nonce, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt large: %v", err)
	}

	got, err := enc.Decrypt(ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt large: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Error("large plaintext roundtrip failed")
	}
}
