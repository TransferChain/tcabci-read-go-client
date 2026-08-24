package tcabcireadgoclient

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
)

const (
	fingerprint = "27dec11ab21f4299d56e5965a8dc28623d2441cb136fbe2b1ea3896c81b1a653"
)

func verifyPeer(rawCerts [][]byte, _ [][]*x509.Certificate, customFingerprint *string) error {
	if customFingerprint != nil && *customFingerprint == "" {
		return nil
	}

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
		if (customFingerprint != nil && *customFingerprint == fingerprints[i]) || fingerprints[i] == fingerprint {
			return nil
		}
	}

	return errors.New("certificate not verified")
}
