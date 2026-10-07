package tcabcireadgoclient

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/hex"
	"errors"
)

const fingerprint = "27dec11ab21f4299d56e5965a8dc28623d2441cb136fbe2b1ea3896c81b1a653"

// An explicit pin replaces the default and binds only the authenticated leaf.
// Appending a public pinned certificate to an unrelated chain cannot satisfy it.
func verifyPeer(rawCerts [][]byte, _ [][]*x509.Certificate, customFingerprint *string) error {
	expected := fingerprint
	if customFingerprint != nil {
		expected = *customFingerprint
	}
	if expected == "" {
		return nil
	} // Explicit opt-out; normal PKI is controlled independently by insecure.
	pin, err := hex.DecodeString(expected)
	if err != nil || len(pin) != sha256.Size || len(rawCerts) == 0 {
		return errors.New("invalid certificate pin")
	}
	actual := sha256.Sum256(rawCerts[0])
	if subtle.ConstantTimeCompare(pin, actual[:]) != 1 {
		return errors.New("certificate not verified")
	}
	return nil
}
