package securemem

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestSecureMemoryRoundTrip(t *testing.T) {
	memory, err := NewSecureMemory()
	if err != nil {
		t.Fatalf("NewSecureMemory() error = %v", err)
	}

	cases := [][]byte{
		[]byte("myP@ssw0rd!"),
		[]byte(""),
		[]byte("Senhas seguras com acentos: caeo"),
		[]byte("sk-" + strings.Repeat("a", 200)),
	}
	for _, tc := range cases {
		sealed, err := memory.Seal(tc)
		if err != nil {
			t.Fatalf("Seal() error = %v", err)
		}
		got, err := memory.Unseal(sealed)
		if err != nil {
			t.Fatalf("Unseal() error = %v", err)
		}
		if !bytes.Equal(got, tc) {
			t.Fatalf("Unseal() = %q, want %q", got, tc)
		}
		Zero(got)
	}
}

func TestSecureMemoryUsesRandomIV(t *testing.T) {
	memory, err := NewSecureMemory()
	if err != nil {
		t.Fatalf("NewSecureMemory() error = %v", err)
	}

	first, err := memory.Seal([]byte("same"))
	if err != nil {
		t.Fatalf("Seal() first error = %v", err)
	}
	second, err := memory.Seal([]byte("same"))
	if err != nil {
		t.Fatalf("Seal() second error = %v", err)
	}
	if bytes.Equal(first.IV, second.IV) {
		t.Fatal("Seal() reused IV")
	}
	if bytes.Equal(first.Ciphertext, second.Ciphertext) {
		t.Fatal("Seal() produced matching ciphertext for matching plaintext")
	}
}

func TestSecureMemoryRejectsTampering(t *testing.T) {
	memory, err := NewSecureMemory()
	if err != nil {
		t.Fatalf("NewSecureMemory() error = %v", err)
	}

	sealed, err := memory.Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	sealed.Ciphertext[0] ^= 0xff
	if _, err := memory.Unseal(sealed); err == nil {
		t.Fatal("Unseal() accepted tampered ciphertext")
	}

	sealed, err = memory.Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	sealed.Tag[0] ^= 0xff
	if _, err := memory.Unseal(sealed); err == nil {
		t.Fatal("Unseal() accepted tampered auth tag")
	}
}

func TestSecureMemoryDestroy(t *testing.T) {
	memory, err := NewSecureMemory()
	if err != nil {
		t.Fatalf("NewSecureMemory() error = %v", err)
	}
	sealed, err := memory.Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}

	esk := memory.esk
	memory.Destroy()
	memory.Destroy()

	if memory.Alive() {
		t.Fatal("Alive() = true after Destroy()")
	}
	if !allZero(esk) {
		t.Fatal("Destroy() did not zero ESK")
	}
	if _, err := memory.Unseal(sealed); !errors.Is(err, ErrDestroyed) {
		t.Fatalf("Unseal() after Destroy() error = %v, want %v", err, ErrDestroyed)
	}
	if _, err := memory.Seal([]byte("again")); !errors.Is(err, ErrDestroyed) {
		t.Fatalf("Seal() after Destroy() error = %v, want %v", err, ErrDestroyed)
	}
}

func TestSecureBytesUseAndDestroy(t *testing.T) {
	memory, err := NewSecureMemory()
	if err != nil {
		t.Fatalf("NewSecureMemory() error = %v", err)
	}
	secure, err := NewSecureBytesWithMemory(memory, []byte("my-api-key"))
	if err != nil {
		t.Fatalf("NewSecureBytesWithMemory() error = %v", err)
	}

	var plainRef []byte
	if err := secure.Use(func(plain []byte) error {
		plainRef = plain
		if string(plain) != "my-api-key" {
			t.Fatalf("Use() plain = %q, want my-api-key", plain)
		}
		return nil
	}); err != nil {
		t.Fatalf("Use() error = %v", err)
	}
	if !allZero(plainRef) {
		t.Fatal("Use() did not zero transient plaintext after callback")
	}

	sealed := secure.SealedForTest()
	if sealed == nil {
		t.Fatal("SealedForTest() = nil before Destroy()")
	}
	secure.Destroy()
	secure.Destroy()

	if secure.Alive() {
		t.Fatal("Alive() = true after Destroy()")
	}
	if !allZero(sealed.IV) || !allZero(sealed.Ciphertext) || !allZero(sealed.Tag) {
		t.Fatal("Destroy() did not zero sealed buffers")
	}
	if _, err := secure.Unwrap(); !errors.Is(err, ErrNotAlive) {
		t.Fatalf("Unwrap() after Destroy() error = %v, want %v", err, ErrNotAlive)
	}
	if err := secure.Use(func([]byte) error { return nil }); !errors.Is(err, ErrNotAlive) {
		t.Fatalf("Use() after Destroy() error = %v, want %v", err, ErrNotAlive)
	}
}

func TestSecureBytesMultipleUnwraps(t *testing.T) {
	secure, err := NewSecureBytes([]byte("reusable"))
	if err != nil {
		t.Fatalf("NewSecureBytes() error = %v", err)
	}
	defer secure.Destroy()

	for range 3 {
		plain, err := secure.Unwrap()
		if err != nil {
			t.Fatalf("Unwrap() error = %v", err)
		}
		if string(plain) != "reusable" {
			t.Fatalf("Unwrap() = %q, want reusable", plain)
		}
		Zero(plain)
	}
}

func allZero(buf []byte) bool {
	for _, b := range buf {
		if b != 0 {
			return false
		}
	}
	return true
}
