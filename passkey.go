package tcabcireadgoclient

import (
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"io"
)

// Match SDK passKey defaults: SHA-512 of four lowercase UUID-v4 ASCII values.
func createClientInstanceID() (string, error) {
	var entropy [16]byte
	var ascii [36]byte
	hasher := sha512.New()
	defer clear(entropy[:])
	defer clear(ascii[:])
	defer hasher.Reset()
	for count := 0; count < 4; count++ {
		if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
			return "", errors.New("cannot generate client instance ID")
		}
		entropy[6] = (entropy[6] & 15) | 64
		entropy[8] = (entropy[8] & 63) | 128
		hex.Encode(ascii[0:8], entropy[0:4])
		hex.Encode(ascii[9:13], entropy[4:6])
		hex.Encode(ascii[14:18], entropy[6:8])
		hex.Encode(ascii[19:23], entropy[8:10])
		hex.Encode(ascii[24:36], entropy[10:16])
		ascii[8], ascii[13], ascii[18], ascii[23] = '-', '-', '-', '-'
		_, _ = hasher.Write(ascii[:])
		clear(entropy[:])
		clear(ascii[:])
	}
	digest := hasher.Sum(nil)
	defer clear(digest)
	return hex.EncodeToString(digest), nil
}
