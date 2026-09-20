// Package did implements did:key operations for DID-based SSO login.
//
// Only ML-DSA-87 (FIPS 204) dids are supported, matching the dai/aifeeds
// wallet. The identifier is base58btc("z") of uvarint(0x300001) ‖ prefixed
// public key bytes; see pq.go for the bundle format and signature
// verification.
package did

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
)

const base58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// base58Decode decodes a base58btc string to bytes.
func base58Decode(str string) ([]byte, error) {
	zeros := 0
	for zeros < len(str) && str[zeros] == '1' {
		zeros++
	}

	num := new(big.Int)
	for i := zeros; i < len(str); i++ {
		idx := strings.IndexByte(base58, str[i])
		if idx < 0 {
			return nil, errors.New("invalid base58 character")
		}
		num.Mul(num, big.NewInt(58))
		num.Add(num, big.NewInt(int64(idx)))
	}

	bytes := num.Bytes()
	out := make([]byte, zeros+len(bytes))
	copy(out[zeros:], bytes)
	return out, nil
}

// base58Encode encodes bytes to a base58btc string.
func base58Encode(input []byte) string {
	zeros := 0
	for zeros < len(input) && input[zeros] == 0 {
		zeros++
	}

	num := new(big.Int).SetBytes(input[zeros:])
	if num.Sign() == 0 {
		return strings.Repeat("1", zeros)
	}

	var out []byte
	mod := new(big.Int)
	for num.Sign() > 0 {
		num.DivMod(num, big.NewInt(58), mod)
		out = append(out, base58[mod.Int64()])
	}

	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}

	return strings.Repeat("1", zeros) + string(out)
}

// encodeVarint encodes an unsigned integer as a multicodec varint.
func encodeVarint(value uint64) []byte {
	var out []byte
	for value > 0x7f {
		out = append(out, byte(value&0x7f)|0x80)
		value >>= 7
	}
	out = append(out, byte(value))
	return out
}

// decodeVarint decodes a multicodec varint from buf at offset, returning the
// value and the number of bytes consumed.
func decodeVarint(buf []byte, offset int) (value uint64, length int, err error) {
	var shift uint
	for i := offset; i < len(buf); i++ {
		b := buf[i]
		value |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return value, i - offset + 1, nil
		}
		shift += 7
	}
	return 0, 0, errors.New("invalid varint")
}

// GenerateNonce returns a 64-char hex random challenge nonce.
func GenerateNonce() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
