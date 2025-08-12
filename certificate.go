package tcabcireadgoclient

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
)

const (
	fingerprint = "dfb514f8d8d8add853dc4f88351b6af64acfb9b8faa3e65fa65de8004e284624"
)

func verifyPeer(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	var fingerprints []string
	for i := 0; i < len(rawCerts); i++ {
		ci, err := x509.ParseCertificate(rawCerts[i])
		if err != nil {
			return err
		}

		sh := sha256.Sum256(ci.Raw)
		fingerprints = append(fingerprints, hex.EncodeToString(sh[:]))
	}

	for i := 0; i < len(fingerprints); i++ {
		if fingerprints[i] == fingerprint {
			return nil
		}
	}

	return errors.New("certificate not verified")
}
