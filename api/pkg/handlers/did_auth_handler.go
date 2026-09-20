package handlers

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/NdoleStudio/httpsms/pkg/did"
	"github.com/NdoleStudio/httpsms/pkg/entities"
	"github.com/NdoleStudio/httpsms/pkg/repositories"
	"github.com/NdoleStudio/httpsms/pkg/requests"
	"github.com/NdoleStudio/httpsms/pkg/telemetry"
	"github.com/NdoleStudio/httpsms/pkg/validators"
	"github.com/davecgh/go-spew/spew"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/palantir/stacktrace"
)

// DIDAuthHandler handles DID-based SSO login requests
type DIDAuthHandler struct {
	handler
	logger         telemetry.Logger
	tracer         telemetry.Tracer
	validator      *validators.DIDAuthHandlerValidator
	userRepository repositories.UserRepository
	nonces         NonceStore
}

// NewDIDAuthHandler creates a new DIDAuthHandler. When nonces is nil an
// in-process store is used; the container passes a shared (Redis) store so
// challenges survive across replicas.
func NewDIDAuthHandler(
	logger telemetry.Logger,
	tracer telemetry.Tracer,
	validator *validators.DIDAuthHandlerValidator,
	userRepository repositories.UserRepository,
	nonces NonceStore,
) (h *DIDAuthHandler) {
	if nonces == nil {
		nonces = newMemoryNonceStore()
	}
	return &DIDAuthHandler{
		logger:         logger.WithService(fmt.Sprintf("%T", h)),
		tracer:         tracer,
		validator:      validator,
		userRepository: userRepository,
		nonces:         nonces,
	}
}

// RegisterRoutes registers the routes for the DIDAuthHandler
// These routes are public (no auth middleware) since they are part of the login flow.
func (h *DIDAuthHandler) RegisterRoutes(router fiber.Router) {
	h.register(router, fiber.MethodPost, "/v1/did-auth/challenge", nil, h.Challenge)
	h.register(router, fiber.MethodPost, "/v1/did-auth/verify", nil, h.Verify)
}

// Challenge issues an authentication challenge for a DID
// @Summary      Issue an authentication challenge
// @Description  Issues a one-time nonce challenge for an ML-DSA-87 did:key identifier. The nonce is signed by the holder of the private key during verification.
// @Tags         DID Auth
// @Accept       json
// @Produce      json
// @Param        payload   body     requests.DIDChallenge  true  "DID to challenge"
// @Success      200       {object} responses.DIDChallengeResponse
// @Failure      400       {object} responses.BadRequest
// @Failure      422       {object} responses.UnprocessableEntity
// @Failure      500       {object} responses.InternalServerError
// @Router       /v1/did-auth/challenge [post]
func (h *DIDAuthHandler) Challenge(c fiber.Ctx) error {
	ctx, span, ctxLogger := h.tracer.StartFromFiberCtxWithLogger(c, h.logger)
	defer span.End()

	var request requests.DIDChallenge
	if err := c.Bind().Body(&request); err != nil {
		msg := fmt.Sprintf("cannot marshall params [%s] into %T", c.OriginalURL(), request)
		ctxLogger.Warn(stacktrace.Propagate(err, msg))
		return h.responseBadRequest(c, err)
	}

	if errors := h.validator.ValidateChallenge(ctx, request.Sanitize()); len(errors) != 0 {
		msg := fmt.Sprintf("validation errors [%s], while issuing challenge [%+#v]", spew.Sdump(errors), request)
		ctxLogger.Warn(stacktrace.NewError(msg))
		return h.responseUnprocessableEntity(c, errors, "validation errors while issuing challenge")
	}

	nonce := did.GenerateNonce()
	id, err := h.nonces.Issue(ctx, request.DID, nonce)
	if err != nil {
		msg := fmt.Sprintf("cannot issue challenge for DID [%s]", request.DID)
		ctxLogger.Error(stacktrace.Propagate(err, msg))
		return h.responseInternalServerError(c)
	}

	return h.responseOK(c, "challenge issued successfully", fiber.Map{
		"id":    id,
		"nonce": nonce,
	})
}

// Verify verifies DID ownership and returns the user
// @Summary      Verify DID ownership and sign in
// @Description  Verifies that the caller signed the issued challenge nonce with the ML-DSA-87 key matching the did:key identifier, then finds or creates the user and returns it (including the API key used for subsequent authenticated requests).
// @Tags         DID Auth
// @Accept       json
// @Produce      json
// @Param        payload   body     requests.DIDVerify  true  "DID, challenge nonce and signature"
// @Success      200       {object} responses.UserResponse
// @Failure      400       {object} responses.BadRequest
// @Failure      401       {object} responses.Unauthorized
// @Failure      422       {object} responses.UnprocessableEntity
// @Failure      500       {object} responses.InternalServerError
// @Router       /v1/did-auth/verify [post]
func (h *DIDAuthHandler) Verify(c fiber.Ctx) error {
	ctx, span, ctxLogger := h.tracer.StartFromFiberCtxWithLogger(c, h.logger)
	defer span.End()

	var request requests.DIDVerify
	if err := c.Bind().Body(&request); err != nil {
		msg := fmt.Sprintf("cannot marshall params [%s] into %T", c.OriginalURL(), request)
		ctxLogger.Warn(stacktrace.Propagate(err, msg))
		return h.responseBadRequest(c, err)
	}

	sanitized := request.Sanitize()
	if errors := h.validator.ValidateVerify(ctx, sanitized); len(errors) != 0 {
		msg := fmt.Sprintf("validation errors [%s], while verifying DID [%+#v]", spew.Sdump(errors), request)
		ctxLogger.Warn(stacktrace.NewError(msg))
		return h.responseUnprocessableEntity(c, errors, "validation errors while verifying DID")
	}

	if err := h.verifyChallenge(ctx, sanitized); err != nil {
		msg := "failed to verify the challenge for the DID"
		ctxLogger.Warn(stacktrace.Propagate(err, msg))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to verify the DID challenge, request a new challenge and try again",
		})
	}

	didValue := sanitized.DID
	user, _, err := h.userRepository.LoadOrStore(ctx, entities.AuthContext{
		ID:    userIDFromDID(didValue),
		Email: didValue,
	})
	if err != nil {
		msg := fmt.Sprintf("cannot load or store user with DID [%s]", didValue)
		ctxLogger.Error(stacktrace.Propagate(err, msg))
		return h.responseInternalServerError(c)
	}

	return h.responseOK(c, "DID verified successfully", user)
}

// userIDFromDID derives the stable user primary key from a did:key. An
// ML-DSA-87 did:key is ~3.5 KB, which exceeds Postgres' btree index row limit
// (2704 bytes) for users_pkey and every UserID foreign-key index, so the DID is
// kept in the unindexed email column and the primary key is a deterministic
// UUIDv5 of the full DID.
func userIDFromDID(didValue string) entities.UserID {
	return entities.UserID(uuid.NewHash(sha256.New(), uuid.Nil, []byte(didValue), 5).String())
}

// verifyChallenge consumes the issued nonce only after the ML-DSA-87 signature
// over it verifies against the DID.
func (h *DIDAuthHandler) verifyChallenge(ctx context.Context, request requests.DIDVerify) error {
	_, span := h.tracer.Start(ctx)
	defer span.End()

	matched, err := h.nonces.Match(ctx, request.ID, request.DID, request.Nonce)
	if err != nil {
		return stacktrace.Propagate(err, "cannot read challenge")
	}
	if !matched {
		return stacktrace.NewError("challenge not found, expired or mismatched")
	}

	if !did.VerifyPQSignature(request.DID, request.Nonce, request.Signature) {
		return stacktrace.NewError("ML-DSA-87 signature verification failed")
	}

	consumed, err := h.nonces.Consume(ctx, request.ID, request.DID, request.Nonce)
	if err != nil {
		return stacktrace.Propagate(err, "cannot consume challenge")
	}
	if !consumed {
		return stacktrace.NewError("challenge already consumed")
	}
	return nil
}
