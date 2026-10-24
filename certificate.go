package tcabcireadgoclient

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"slices"
	"sync"
)

var (
	// fingerprintsMu guards defaultFingerprints: callers may change the list
	// while TLS handshakes read it concurrently.
	fingerprintsMu sync.RWMutex

	// defaultFingerprints are the hex SHA-256 pins of the DER certificates
	// accepted when no custom fingerprint or certificate is given. Old pins
	// stay in the list so earlier server certificates keep working. The
	// Let's Encrypt intermediates keep clients working across leaf renewals.
	defaultFingerprints = []string{
		// Earlier *.transferchain.io leaf.
		"27dec11ab21f4299d56e5965a8dc28623d2441cb136fbe2b1ea3896c81b1a653",
		// *.transferchain.io ECDSA leaf (YE2), 2026-10-22 → 2027-01-20.
		"75f6d90040cab176418687fdad09adb051010829f31008209c6cf9e8f7b52ef6",
		// *.transferchain.io RSA leaf (YR1), 2026-10-22 → 2027-01-20.
		"5a927cc80015a0c096b52aa25156f9507187ad499576e6a9d7c62ccf284a71c9",
		// Let's Encrypt YE2 intermediate (ECDSA), until 2028-09-02.
		"97658de8c68dfa98ace1e5028a63d54a1aae911b3e21471076c6850cd08cbab4",
		// Let's Encrypt YR1 intermediate (RSA), until 2028-09-02.
		"13949634d99cd6fd6aa80bc034fefacceb1969feef986586713ecdbb05758d3f",
	}
)

// SetCustomFingerprint replaces the default pin list used by clients created
// with a nil fingerprint and no certificate. The slice is copied, so later
// changes to the caller's slice do not change the pins.
func SetCustomFingerprint(fingerprints []string) {
	fingerprintsMu.Lock()
	defer fingerprintsMu.Unlock()

	defaultFingerprints = slices.Clone(fingerprints)
}

// AddFingerprints appends one hex SHA-256 pin to the default pin list.
func AddFingerprints(fingerprint string) {
	fingerprintsMu.Lock()
	defer fingerprintsMu.Unlock()

	defaultFingerprints = append(defaultFingerprints, fingerprint)
}

// GetFormattedFingerprints returns a copy of the default pin list; changing
// the returned slice does not change the pins.
func GetFormattedFingerprints() []string {
	fingerprintsMu.RLock()
	defer fingerprintsMu.RUnlock()

	return slices.Clone(defaultFingerprints)
}

// verifyPeer checks that the server's certificate chain contains a pinned
// certificate.
//
//   - customFingerprint points to "": pinning is disabled (explicit opt-out;
//     normal PKI is controlled independently by insecure).
//   - customFingerprint is non-empty: only that pin is accepted; it replaces
//     the default list.
//   - customFingerprint is nil: the default list is accepted. The default
//     list is read on every handshake, so later list changes apply to
//     existing clients.
//
// A pin matches only the leaf or a certificate that signs the chain down to
// the leaf (see signedChain). Appending a public pinned certificate to an
// unrelated chain cannot satisfy it.
//
// It returns "invalid certificate pin" for a malformed custom pin or an empty
// chain, and "certificate not verified" when no pin matches.
func verifyPeer(peerCerts []*x509.Certificate, customFingerprint *string) error {
	pins := []string{}
	if customFingerprint != nil {
		if *customFingerprint == "" {
			return nil
		}

		// A malformed custom pin is a caller mistake and is reported as such;
		// malformed entries in the default list simply never match.
		if _, err := decodePin(*customFingerprint); err != nil {
			return errors.New("invalid certificate pin")
		}

		pins = append(pins, *customFingerprint)
	} else {
		pins = GetFormattedFingerprints()
	}

	if len(peerCerts) == 0 {
		return errors.New("invalid certificate pin")
	}

	for _, cert := range signedChain(peerCerts) {
		actual := sha256.Sum256(cert.Raw)

		for _, pin := range pins {
			expected, err := decodePin(pin)
			if err != nil {
				continue
			}

			if subtle.ConstantTimeCompare(expected, actual[:]) == 1 {
				return nil
			}
		}
	}

	return errors.New("certificate not verified")
}

// decodePin converts a hex SHA-256 pin (either case) to bytes.
//
// It returns an error when the pin is not hex or not 32 bytes long.
func decodePin(pin string) ([]byte, error) {
	decoded, err := hex.DecodeString(pin)
	if err != nil {
		return nil, err
	}

	if len(decoded) != sha256.Size {
		return nil, errors.New("invalid certificate pin length")
	}

	return decoded, nil
}

// signedChain returns the leaf followed by each certificate that signed the
// one before it. The walk stops at the first broken link: anything after it
// does not vouch for the leaf.
//
// The leaf needs no signature check here, because the TLS handshake already
// proves the server holds the leaf's private key, even with insecure.
// CheckSignatureFrom also rejects parents that are not CAs, so a pinned leaf
// cannot be used as a fake issuer.
func signedChain(certs []*x509.Certificate) []*x509.Certificate {
	for i := 1; i < len(certs); i++ {
		if certs[i-1].CheckSignatureFrom(certs[i]) != nil {
			return certs[:i]
		}
	}

	return certs
}
