package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NdoleStudio/httpsms/pkg/did"
	"github.com/NdoleStudio/httpsms/pkg/entities"
	"github.com/NdoleStudio/httpsms/pkg/repositories"
	"github.com/NdoleStudio/httpsms/pkg/requests"
	"github.com/NdoleStudio/httpsms/pkg/telemetry"
	"github.com/NdoleStudio/httpsms/pkg/validators"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/gofiber/fiber/v3"
	"github.com/hirosassa/zerodriver"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errUserNotFound = errors.New("user not found")

// fakeUserRepository stores users in memory for testing
type fakeUserRepository struct {
	users map[entities.UserID]*entities.User
}

func (r *fakeUserRepository) Store(_ context.Context, user *entities.User) error {
	r.users[user.ID] = user
	return nil
}

func (r *fakeUserRepository) Update(_ context.Context, user *entities.User) error {
	r.users[user.ID] = user
	return nil
}

func (r *fakeUserRepository) LoadAuthContext(_ context.Context, apiKey string) (entities.AuthContext, error) {
	for _, user := range r.users {
		if user.APIKey == apiKey {
			return entities.AuthContext{ID: user.ID, Email: user.Email}, nil
		}
	}
	return entities.AuthContext{}, errUserNotFound
}

func (r *fakeUserRepository) Load(_ context.Context, userID entities.UserID) (*entities.User, error) {
	user, ok := r.users[userID]
	if !ok {
		return nil, errUserNotFound
	}
	return user, nil
}

func (r *fakeUserRepository) RotateAPIKey(_ context.Context, userID entities.UserID) (*entities.User, error) {
	user, ok := r.users[userID]
	if !ok {
		return nil, errUserNotFound
	}
	user.APIKey = "uk_rotated"
	return user, nil
}

func (r *fakeUserRepository) LoadOrStore(_ context.Context, authUser entities.AuthContext) (*entities.User, bool, error) {
	if user, ok := r.users[authUser.ID]; ok {
		return user, false, nil
	}
	user := &entities.User{
		ID:     authUser.ID,
		Email:  authUser.Email,
		APIKey: "uk_testkey",
	}
	r.users[authUser.ID] = user
	return user, true, nil
}

func (r *fakeUserRepository) LoadBySubscriptionID(_ context.Context, subscriptionID string) (*entities.User, error) {
	return nil, errUserNotFound
}

func (r *fakeUserRepository) LoadByEmail(_ context.Context, email string) (*entities.User, error) {
	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, errUserNotFound
}

func (r *fakeUserRepository) Delete(_ context.Context, _ *entities.User) error {
	return nil
}

// testBufferedLogger creates a zerolog logger that writes to a buffer
func testBufferedLogger() telemetry.Logger {
	var buffer bytes.Buffer
	l := zerolog.New(&buffer).With().Timestamp().Logger()
	driver := &zerodriver.Logger{Logger: &l}
	return telemetry.NewZerologLogger("", map[string]string{}, driver, nil)
}

// newDIDAuthTestHandler creates a DIDAuthHandler wired to a fake repository
func newDIDAuthTestHandler(repo repositories.UserRepository) *DIDAuthHandler {
	logger := testBufferedLogger()
	tracer := telemetry.NewOtelLogger("", logger)
	validator := validators.NewDIDAuthHandlerValidator(logger, tracer)
	return NewDIDAuthHandler(logger, tracer, validator, repo, nil)
}

// performRequest runs a request against the handler and returns the status and decoded body
func performRequest(t *testing.T, h *DIDAuthHandler, method, path string, payload any) (*http.Response, fiber.Map) {
	t.Helper()
	app := fiber.New()
	h.RegisterRoutes(app)

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	var decoded fiber.Map
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))
	return resp, decoded
}

// pqTestUser is an in-memory ML-DSA-87 identity that can sign challenge nonces
// in the same bapp wire format the wallet uses.
type pqTestUser struct {
	did string
	sk  *mldsa87.PrivateKey
}

func newPQTestUser(t *testing.T) *pqTestUser {
	t.Helper()
	pub, sk, err := mldsa87.GenerateKey(rand.Reader)
	require.NoError(t, err)

	pubBytes := pub.Bytes()
	prefixed := make([]byte, 0, 2+3+len(pubBytes))
	prefixed = append(prefixed, 0x05, 0x01, 0x01, 0x01)
	prefixed = append(prefixed, byte(len(pubBytes)>>8), byte(len(pubBytes)))
	prefixed = append(prefixed, pubBytes...)

	return &pqTestUser{did: did.DIDFromPQPub(prefixed), sk: sk}
}

func (u *pqTestUser) sign(t *testing.T, nonceHex string) string {
	t.Helper()
	nonce, err := hex.DecodeString(nonceHex)
	require.NoError(t, err)

	txHash := domainHash("BIGTANGLE-PQ-TX-v1", nonce)
	mlMsg := domainHash("MLDSA-SIG-DOMAIN", txHash)
	sig := make([]byte, mldsa87.SignatureSize)
	require.NoError(t, mldsa87.SignTo(u.sk, mlMsg, nil, false, sig))

	bundle := make([]byte, 0, 2+3+len(sig))
	bundle = append(bundle, 0x01, 0x01, 0x01)
	bundle = append(bundle, byte(len(sig)>>8), byte(len(sig)))
	bundle = append(bundle, sig...)
	return hex.EncodeToString(bundle)
}

func domainHash(domain string, data []byte) []byte {
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write(data)
	return h.Sum(nil)
}

func TestDIDAuthHandler_ChallengeAndVerify(t *testing.T) {
	// Arrange
	repo := &fakeUserRepository{users: map[entities.UserID]*entities.User{}}
	h := newDIDAuthTestHandler(repo)
	user := newPQTestUser(t)

	// Act: challenge
	challengeResp, challenge := performRequest(t, h, fiber.MethodPost, "/v1/did-auth/challenge", requests.DIDChallenge{DID: user.did})
	require.Equal(t, 200, challengeResp.StatusCode)
	challengeData := challenge["data"].(map[string]interface{})
	require.NotEmpty(t, challengeData["nonce"])
	require.NotEmpty(t, challengeData["id"])

	nonce := challengeData["nonce"].(string)
	id := challengeData["id"].(string)

	// Act: verify
	verifyResp, decoded := performRequest(t, h, fiber.MethodPost, "/v1/did-auth/verify", requests.DIDVerify{
		ID:        id,
		DID:       user.did,
		Nonce:     nonce,
		Signature: user.sign(t, nonce),
		Scheme:    "mldsa",
	})
	require.Equal(t, 200, verifyResp.StatusCode, "verify should succeed: %+#v", decoded)

	created := decoded["data"].(map[string]interface{})
	assert.Equal(t, user.did, created["email"])
	require.NotEmpty(t, created["id"])
	require.NotEmpty(t, created["api_key"])

	_, err := repo.Load(context.Background(), userIDFromDID(user.did))
	assert.Nil(t, err)
}

func TestDIDAuthHandler_ChallengeRejectsInvalidDID(t *testing.T) {
	repo := &fakeUserRepository{users: map[entities.UserID]*entities.User{}}
	h := newDIDAuthTestHandler(repo)

	resp, _ := performRequest(t, h, fiber.MethodPost, "/v1/did-auth/challenge", requests.DIDChallenge{DID: "did:key:z6Mkabc"})
	assert.Equal(t, 422, resp.StatusCode)
}

func TestDIDAuthHandler_VerifyRejectsBadChallenge(t *testing.T) {
	repo := &fakeUserRepository{users: map[entities.UserID]*entities.User{}}
	h := newDIDAuthTestHandler(repo)
	user := newPQTestUser(t)

	nonce := "deadbeef"
	resp, _ := performRequest(t, h, fiber.MethodPost, "/v1/did-auth/verify", requests.DIDVerify{
		ID:        "some-unknown-challenge-id",
		DID:       user.did,
		Nonce:     nonce,
		Signature: user.sign(t, nonce),
		Scheme:    "mldsa",
	})

	assert.Equal(t, 401, resp.StatusCode)
	assert.Len(t, repo.users, 0)
}

func TestDIDAuthHandler_VerifyRejectsWrongSignatureWithoutBurningChallenge(t *testing.T) {
	repo := &fakeUserRepository{users: map[entities.UserID]*entities.User{}}
	h := newDIDAuthTestHandler(repo)
	user := newPQTestUser(t)
	attacker := newPQTestUser(t)

	_, challenge := performRequest(t, h, fiber.MethodPost, "/v1/did-auth/challenge", requests.DIDChallenge{DID: user.did})
	challengeData := challenge["data"].(map[string]interface{})
	nonce := challengeData["nonce"].(string)
	id := challengeData["id"].(string)

	// A signature from another key must fail...
	badResp, _ := performRequest(t, h, fiber.MethodPost, "/v1/did-auth/verify", requests.DIDVerify{
		ID:        id,
		DID:       user.did,
		Nonce:     nonce,
		Signature: attacker.sign(t, nonce),
		Scheme:    "mldsa",
	})
	assert.Equal(t, 401, badResp.StatusCode)
	assert.Len(t, repo.users, 0)

	// ...and must not consume the challenge, so the real key can still use it.
	goodResp, _ := performRequest(t, h, fiber.MethodPost, "/v1/did-auth/verify", requests.DIDVerify{
		ID:        id,
		DID:       user.did,
		Nonce:     nonce,
		Signature: user.sign(t, nonce),
		Scheme:    "mldsa",
	})
	assert.Equal(t, 200, goodResp.StatusCode)
}

func TestDIDAuthHandler_VerifyRejectsReplayedChallenge(t *testing.T) {
	repo := &fakeUserRepository{users: map[entities.UserID]*entities.User{}}
	h := newDIDAuthTestHandler(repo)
	user := newPQTestUser(t)

	_, challenge := performRequest(t, h, fiber.MethodPost, "/v1/did-auth/challenge", requests.DIDChallenge{DID: user.did})
	challengeData := challenge["data"].(map[string]interface{})
	nonce := challengeData["nonce"].(string)
	id := challengeData["id"].(string)
	payload := requests.DIDVerify{ID: id, DID: user.did, Nonce: nonce, Signature: user.sign(t, nonce), Scheme: "mldsa"}

	first, _ := performRequest(t, h, fiber.MethodPost, "/v1/did-auth/verify", payload)
	require.Equal(t, 200, first.StatusCode)

	second, _ := performRequest(t, h, fiber.MethodPost, "/v1/did-auth/verify", payload)
	assert.Equal(t, 401, second.StatusCode)
}

func TestDIDAuthHandler_VerifyRequiresFields(t *testing.T) {
	repo := &fakeUserRepository{users: map[entities.UserID]*entities.User{}}
	h := newDIDAuthTestHandler(repo)

	resp, _ := performRequest(t, h, fiber.MethodPost, "/v1/did-auth/verify", requests.DIDVerify{})
	assert.Equal(t, 422, resp.StatusCode)
}

var _ repositories.UserRepository = (*fakeUserRepository)(nil)
