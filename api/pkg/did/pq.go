package did

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/cloudflare/circl/sign/slhdsa"
)

// ML-DSA-87 (FIPS 204) challenge verification, wire-compatible with
// bigtangle-ts PQKey / SignatureBundle. The DID encodes
// uvarint(MulticodecMLDSA87) ‖ prefixedPubBytes, where prefixedPubBytes is
// 0x05 ‖ KeyBundle; the challenge signature is a serialized SignatureBundle
// over a double domain-separated SHA-256 of the nonce.
const (
	// MulticodecMLDSA87 is the private-use multicodec for ML-DSA-87 did:key.
	MulticodecMLDSA87 = 0x300001

	pqPubPrefix      = 0x05
	bundleVersion    = 1
	algMLDSA87       = 1
	algSLHDSA256s    = 2
	txDomain         = "BIGTANGLE-PQ-TX-v1"
	mldsaSigDomain   = "MLDSA-SIG-DOMAIN"
	slhdsaSigDomain  = "SLHDSA-SIG-DOMAIN"
	keyBundleMinSize = 2
)

// domainSeparatedHash is sha256(domain ‖ data), matching bigtangle-ts.
func domainSeparatedHash(data []byte, domain string) []byte {
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write(data)
	return h.Sum(nil)
}

// parseBundle decodes a KeyBundle or SignatureBundle:
// version(1) ‖ count(1) ‖ { algorithm(1) ‖ length(2, big-endian) ‖ bytes }*.
func parseBundle(raw []byte) (map[byte][]byte, error) {
	if len(raw) < keyBundleMinSize {
		return nil, errors.New("bundle too short")
	}
	if raw[0] != bundleVersion {
		return nil, errors.New("unsupported bundle version")
	}

	entries := make(map[byte][]byte, raw[1])
	offset := keyBundleMinSize
	for i := 0; i < int(raw[1]); i++ {
		if offset+3 > len(raw) {
			return nil, errors.New("truncated bundle entry")
		}
		algorithm := raw[offset]
		length := int(raw[offset+1])<<8 | int(raw[offset+2])
		offset += 3
		if offset+length > len(raw) {
			return nil, errors.New("truncated bundle bytes")
		}
		entries[algorithm] = raw[offset : offset+length]
		offset += length
	}
	return entries, nil
}

// pqPubFromDID extracts the prefixed public key bytes from a did:key:z… id.
func pqPubFromDID(did string) ([]byte, error) {
	parts := strings.Split(did, ":")
	if len(parts) != 3 || parts[0] != "did" || parts[1] != "key" {
		return nil, errors.New("invalid DID format, expected did:key:...")
	}
	enc := parts[2]
	if len(enc) == 0 || enc[0] != 'z' {
		return nil, errors.New("only base58btc DIDs supported")
	}

	decoded, err := base58Decode(enc[1:])
	if err != nil {
		return nil, err
	}

	codec, length, err := decodeVarint(decoded, 0)
	if err != nil {
		return nil, err
	}
	if codec != MulticodecMLDSA87 {
		return nil, errors.New("unsupported multicodec")
	}

	return decoded[length:], nil
}

// IsValidPQDID reports whether did is a well-formed ML-DSA-87 did:key.
func IsValidPQDID(did string) bool {
	prefixed, err := pqPubFromDID(did)
	if err != nil || len(prefixed) < 2 || prefixed[0] != pqPubPrefix {
		return false
	}
	entries, err := parseBundle(prefixed[1:])
	if err != nil {
		return false
	}
	_, ok := entries[algMLDSA87]
	return ok
}

// VerifyPQSignature verifies a bapp-style ML-DSA-87 challenge signature
// (hex-serialized SignatureBundle) over the hex nonce against did.
func VerifyPQSignature(did string, nonceHex string, sigBundleHex string) bool {
	prefixed, err := pqPubFromDID(did)
	if err != nil || len(prefixed) < 2 || prefixed[0] != pqPubPrefix {
		return false
	}
	keyEntries, err := parseBundle(prefixed[1:])
	if err != nil {
		return false
	}
	pubBytes, ok := keyEntries[algMLDSA87]
	if !ok {
		return false
	}

	sigRaw, err := hex.DecodeString(sigBundleHex)
	if err != nil {
		return false
	}
	sigEntries, err := parseBundle(sigRaw)
	if err != nil {
		return false
	}
	sig, ok := sigEntries[algMLDSA87]
	if !ok {
		return false
	}

	nonce, err := hex.DecodeString(nonceHex)
	if err != nil {
		return false
	}

	var pub mldsa87.PublicKey
	if err := pub.UnmarshalBinary(pubBytes); err != nil {
		return false
	}

	txHash := domainSeparatedHash(nonce, txDomain)
	mlMsg := domainSeparatedHash(txHash, mldsaSigDomain)
	if !mldsa87.Verify(&pub, mlMsg, nil, sig) {
		return false
	}

	// Dual keys also carry SLH-DSA-SHA2-256s; verify it when present, matching
	// bigtangle-ts PQKey.verify.
	if slhPub, ok := keyEntries[algSLHDSA256s]; ok {
		slhSig, ok := sigEntries[algSLHDSA256s]
		if !ok {
			return false
		}
		var slhKey slhdsa.PublicKey
		slhKey.ID = slhdsa.SHA2_256s
		if err := slhKey.UnmarshalBinary(slhPub); err != nil {
			return false
		}
		slhMsg := domainSeparatedHash(txHash, slhdsaSigDomain)
		if !slhdsa.Verify(&slhKey, slhdsa.NewMessage(slhMsg), slhSig, nil) {
			return false
		}
	}
	return true
}

// DIDFromPQPub builds a did:key:z… identifier from prefixed PQ public key bytes.
func DIDFromPQPub(prefixedPub []byte) string {
	raw := append(encodeVarint(MulticodecMLDSA87), prefixedPub...)
	return "did:key:z" + base58Encode(raw)
}
