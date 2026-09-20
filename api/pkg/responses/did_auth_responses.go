package responses

// DIDChallengeResponse is the response with the challenge nonce
type DIDChallengeResponse struct {
	response
	Data DIDChallengeData `json:"data"`
}

// DIDChallengeData holds the challenge identifier and nonce
type DIDChallengeData struct {
	ID    string `json:"id" example:"32343a19-da5e-4b1b-a767-3298a73703ca"`
	Nonce string `json:"nonce" example:"9f86d0818..."`
}