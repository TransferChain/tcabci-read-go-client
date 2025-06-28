package tcabcireadgoclient

import (
	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/ed25519"
)

// Sign ..
func Sign(signKey []byte, data []byte) []byte {
	sign := ed25519.Sign(signKey, data[:])
	return sign
}

func KeyFillFromSeed(seed [32]byte) ([32]byte, []byte, string, error) {
	publicKeyEncrypt := make([]byte, 32)
	privateKeySign := make([]byte, ed25519.PrivateKeySize)
	publicKeySign := make([]byte, ed25519.PublicKeySize)

	pke := new([32]byte)
	curve25519.ScalarBaseMult(pke, &seed)

	for i, pp := range pke {
		publicKeyEncrypt[i] = pp
	}

	privateKeySign = ed25519.NewKeyFromSeed(seed[:])
	copy(publicKeySign, privateKeySign[32:])

	adr := make([]byte, 0, 64)
	adr = append(adr, publicKeySign[:]...)
	adr = append(adr, publicKeyEncrypt[:32]...)
	address := base58.Encode(adr)

	return seed, privateKeySign, address, nil
}
