package requests

import "strings"

// DIDChallenge is the request to issue an authentication challenge for a DID
type DIDChallenge struct {
	request
	// DID is the did:key identifier to challenge
	DID string `json:"did" example:"did:key:z..."`
}

// Sanitize normalizes the DIDChallenge request
func (input *DIDChallenge) Sanitize() DIDChallenge {
	input.DID = strings.TrimSpace(input.DID)
	return *input
}

// DIDVerify is the request to verify DID ownership and sign in
type DIDVerify struct {
	request
	// ID is the challenge ID returned by the challenge endpoint
	ID string `json:"id" example:"32343a19-da5e-4b1b-a767-3298a73703ca"`
	// DID is the did:key identifier to authenticate
	DID string `json:"did" example:"did:key:z..."`
	// Nonce is the challenge nonce returned by the challenge endpoint
	Nonce string `json:"nonce" example:"9f86d0818..."`
	// Signature is the hex-serialized ML-DSA-87 SignatureBundle over the nonce
	Signature string `json:"signature" example:"0101..."`
	// Scheme identifies the signature algorithm; only "mldsa" is supported
	Scheme string `json:"scheme" example:"mldsa"`
}

// Sanitize normalizes the DIDVerify request
func (input *DIDVerify) Sanitize() DIDVerify {
	input.ID = strings.TrimSpace(input.ID)
	input.DID = strings.TrimSpace(input.DID)
	input.Nonce = strings.TrimSpace(strings.ToLower(input.Nonce))
	input.Signature = strings.TrimSpace(strings.ToLower(input.Signature))
	input.Scheme = strings.TrimSpace(strings.ToLower(input.Scheme))
	return *input
}
