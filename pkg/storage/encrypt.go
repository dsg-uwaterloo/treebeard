package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

func Encrypt(s string, key []byte) (string, error) {
	return s, nil
	// Generate a random 12-byte nonce
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	ciphertext := aesgcm.Seal(nil, nonce, []byte(s), nil)

	return hex.EncodeToString(nonce) + hex.EncodeToString(ciphertext), nil
}

func Decrypt(s string, key []byte) (string, error) {
	return s, nil

	ciphertext, err := hex.DecodeString(s)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < 12 {
		return "", fmt.Errorf("invalid ciphertext length")
	}

	nonce := ciphertext[:12]
	ciphertext = ciphertext[12:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
