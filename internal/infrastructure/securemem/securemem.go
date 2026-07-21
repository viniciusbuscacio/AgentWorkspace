// Package securemem ports AW2's in-memory sealing primitives to Go.
//
// It keeps sensitive bytes encrypted at rest in process memory behind an
// ephemeral session key (ESK), and zeroes transient plaintext buffers when a
// callback returns. This is best-effort memory hygiene: Go's runtime, swap, core
// dumps, and caller-created strings/copies can still retain data outside this
// package's control.
package securemem

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"sync"
)

const eskSize = 32

var (
	ErrDestroyed = errors.New("secure memory has been destroyed")
	ErrNotAlive  = errors.New("secure value has been destroyed")
)

type SealedValue struct {
	IV         []byte
	Ciphertext []byte
	Tag        []byte
}

type SecureMemory struct {
	mu        sync.Mutex
	esk       []byte
	destroyed bool
}

func NewSecureMemory() (*SecureMemory, error) {
	esk, err := randomBytes(eskSize)
	if err != nil {
		return nil, err
	}
	return &SecureMemory{esk: esk}, nil
}

func (m *SecureMemory) Seal(plaintext []byte) (SealedValue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.destroyed {
		return SealedValue{}, ErrDestroyed
	}
	gcm, err := m.gcmLocked()
	if err != nil {
		return SealedValue{}, err
	}
	iv, err := randomBytes(gcm.NonceSize())
	if err != nil {
		return SealedValue{}, err
	}
	ciphertextWithTag := gcm.Seal(nil, iv, plaintext, nil)
	tagStart := len(ciphertextWithTag) - gcm.Overhead()

	return SealedValue{
		IV:         iv,
		Ciphertext: append([]byte(nil), ciphertextWithTag[:tagStart]...),
		Tag:        append([]byte(nil), ciphertextWithTag[tagStart:]...),
	}, nil
}

func (m *SecureMemory) Unseal(sealed SealedValue) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.destroyed {
		return nil, ErrDestroyed
	}
	gcm, err := m.gcmLocked()
	if err != nil {
		return nil, err
	}
	ciphertextWithTag := make([]byte, 0, len(sealed.Ciphertext)+len(sealed.Tag))
	ciphertextWithTag = append(ciphertextWithTag, sealed.Ciphertext...)
	ciphertextWithTag = append(ciphertextWithTag, sealed.Tag...)
	defer Zero(ciphertextWithTag)

	return gcm.Open(nil, sealed.IV, ciphertextWithTag, nil)
}

func (m *SecureMemory) Destroy() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.destroyed {
		return
	}
	Zero(m.esk)
	m.destroyed = true
}

func (m *SecureMemory) Alive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return !m.destroyed
}

func (m *SecureMemory) gcmLocked() (cipher.AEAD, error) {
	block, err := aes.NewCipher(m.esk)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

type SecureBytes struct {
	mu     sync.Mutex
	memory *SecureMemory
	sealed *SealedValue
}

func NewSecureBytes(plaintext []byte) (*SecureBytes, error) {
	return NewSecureBytesWithMemory(DefaultMemory(), plaintext)
}

func NewSecureBytesWithMemory(memory *SecureMemory, plaintext []byte) (*SecureBytes, error) {
	if memory == nil {
		return nil, ErrDestroyed
	}
	sealed, err := memory.Seal(plaintext)
	if err != nil {
		return nil, err
	}
	return &SecureBytes{memory: memory, sealed: &sealed}, nil
}

func (s *SecureBytes) Use(fn func([]byte) error) error {
	if fn == nil {
		return nil
	}
	plain, err := s.Unwrap()
	if err != nil {
		return err
	}
	defer Zero(plain)
	return fn(plain)
}

func (s *SecureBytes) Unwrap() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sealed == nil {
		return nil, ErrNotAlive
	}
	return s.memory.Unseal(*s.sealed)
}

func (s *SecureBytes) Destroy() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sealed == nil {
		return
	}
	Zero(s.sealed.IV)
	Zero(s.sealed.Ciphertext)
	Zero(s.sealed.Tag)
	s.sealed = nil
}

func (s *SecureBytes) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sealed != nil
}

func (s *SecureBytes) SealedForTest() *SealedValue {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sealed
}

func Zero(buf []byte) {
	for i := range buf {
		buf[i] = 0
	}
}

func randomBytes(n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

var defaultMemory = func() *SecureMemory {
	memory, err := NewSecureMemory()
	if err != nil {
		panic(err)
	}
	return memory
}()

func DefaultMemory() *SecureMemory {
	return defaultMemory
}
