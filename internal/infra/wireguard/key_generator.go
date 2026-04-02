package wireguard

import (
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/crypto/curve25519"

	"vpn-backend/internal/domain"
)

const keySize = 32

type KeyGenerator struct{}

var _ domain.KeyGenerator = (*KeyGenerator)(nil)

func NewKeyGenerator() *KeyGenerator {
	return &KeyGenerator{}
}

func (g *KeyGenerator) Generate() (*domain.KeyPair, error) {
	privateKey, err := generatePrivateKey()
	if err != nil {
		return nil, err
	}

	publicKey, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		return nil, err
	}

	return &domain.KeyPair{
		PrivateKey: encodeKey(privateKey[:]),
		PublicKey:  encodeKey(publicKey),
	}, nil
}

func generatePrivateKey() ([keySize]byte, error) {
	var privateKey [keySize]byte

	if _, err := rand.Read(privateKey[:]); err != nil {
		return [keySize]byte{}, err
	}

	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	return privateKey, nil
}

func encodeKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}
