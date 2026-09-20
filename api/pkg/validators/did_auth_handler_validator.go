package validators

import (
	"context"
	"fmt"
	"net/url"

	"github.com/NdoleStudio/httpsms/pkg/did"
	"github.com/NdoleStudio/httpsms/pkg/requests"
	"github.com/NdoleStudio/httpsms/pkg/telemetry"
)

// DIDAuthHandlerValidator validates models used in handlers.DIDAuthHandler
type DIDAuthHandlerValidator struct {
	validator
	logger telemetry.Logger
	tracer telemetry.Tracer
}

// NewDIDAuthHandlerValidator creates a new handlers.DIDAuthHandler validator
func NewDIDAuthHandlerValidator(
	logger telemetry.Logger,
	tracer telemetry.Tracer,
) (v *DIDAuthHandlerValidator) {
	return &DIDAuthHandlerValidator{
		logger: logger.WithService(fmt.Sprintf("%T", v)),
		tracer: tracer,
	}
}

// ValidateChallenge validates requests.DIDChallenge
func (validator *DIDAuthHandlerValidator) ValidateChallenge(_ context.Context, request requests.DIDChallenge) url.Values {
	return validator.validateDID(request.DID)
}

// ValidateVerify validates requests.DIDVerify
func (validator *DIDAuthHandlerValidator) ValidateVerify(_ context.Context, request requests.DIDVerify) url.Values {
	errors := validator.validateDID(request.DID)

	if request.ID == "" {
		errors.Add("id", "The id field is required")
	}

	if request.Nonce == "" {
		errors.Add("nonce", "The nonce field is required")
	}

	if request.Signature == "" {
		errors.Add("signature", "The signature field is required")
	}

	if request.Scheme != "" && request.Scheme != "mldsa" {
		errors.Add("scheme", "The scheme field must be \"mldsa\"")
	}

	return errors
}

// validateDID validates a did:key identifier
func (validator *DIDAuthHandlerValidator) validateDID(value string) url.Values {
	errors := url.Values{}

	if value == "" {
		errors.Add("did", "The did field is required")
		return errors
	}

	if !did.IsValidPQDID(value) {
		errors.Add("did", "The did field must be a valid ML-DSA-87 did:key identifier")
	}

	return errors
}
