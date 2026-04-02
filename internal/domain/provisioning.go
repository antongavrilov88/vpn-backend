package domain

import "context"

type KeyPair struct {
	PublicKey  string
	PrivateKey string
}

type KeyGenerator interface {
	Generate() (*KeyPair, error)
}

type PrivateKeyCipher interface {
	Encrypt(ctx context.Context, plaintext string) (string, error)
	Decrypt(ctx context.Context, ciphertext string) (string, error)
}

type IPAllocator interface {
	AllocateNext(ctx context.Context) (string, error)
}
