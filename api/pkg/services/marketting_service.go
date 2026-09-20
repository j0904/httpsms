package services

import (
	"context"
	"fmt"

	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"

	"github.com/NdoleStudio/httpsms/pkg/entities"
	"github.com/NdoleStudio/httpsms/pkg/repositories"
	"github.com/NdoleStudio/httpsms/pkg/telemetry"
	plunk "github.com/NdoleStudio/plunk-go"
	"github.com/gofiber/fiber/v3"
	"github.com/palantir/stacktrace"
)

// MarketingService is handles marketing requests
type MarketingService struct {
	logger         telemetry.Logger
	tracer         telemetry.Tracer
	userRepository repositories.UserRepository
	plunkClient    *plunk.Client
}

// NewMarketingService creates a new instance of the MarketingService
func NewMarketingService(
	logger telemetry.Logger,
	tracer telemetry.Tracer,
	userRepository repositories.UserRepository,
	plunkClient *plunk.Client,
) *MarketingService {
	return &MarketingService{
		logger:         logger.WithService(fmt.Sprintf("%T", &MarketingService{})),
		tracer:         tracer,
		userRepository: userRepository,
		plunkClient:    plunkClient,
	}
}

// DeleteContact a user if exists as a contact
func (service *MarketingService) DeleteContact(ctx context.Context, email string) error {
	ctx, span, ctxLogger := service.tracer.StartWithLogger(ctx, service.logger)
	defer span.End()

	response, _, err := service.plunkClient.Contacts.List(ctx, map[string]string{"search": email})
	if err != nil {
		return service.tracer.WrapErrorSpan(span, stacktrace.Propagate(err, fmt.Sprintf("cannot search for contact with email [%s]", email)))
	}

	if len(response.Data) == 0 {
		ctxLogger.Info(fmt.Sprintf("no contact found with email [%s], skipping deletion", email))
		return nil
	}

	contact := response.Data[0]
	if _, err = service.plunkClient.Contacts.Delete(ctx, contact.ID); err != nil {
		return service.tracer.WrapErrorSpan(span, stacktrace.Propagate(err, fmt.Sprintf("cannot delete user with ID [%s] from contacts", contact.Data[string(semconv.EnduserIDKey)])))
	}

	ctxLogger.Info(fmt.Sprintf("deleted user with ID [%s] from as marketting contact with ID [%s]", contact.Data[string(semconv.EnduserIDKey)], contact.ID))
	return nil
}

// CreateContact adds a new user on the onboarding automation.
func (service *MarketingService) CreateContact(ctx context.Context, userID entities.UserID) error {
	ctx, span, ctxLogger := service.tracer.StartWithLogger(ctx, service.logger)
	defer span.End()

	user, err := service.userRepository.Load(ctx, userID)
	if err != nil {
		msg := fmt.Sprintf("cannot load user with id [%s]", userID)
		return service.tracer.WrapErrorSpan(span, stacktrace.Propagate(err, msg))
	}

	data := fiber.Map{}
	data[string(semconv.ServiceNameKey)] = "httpsms.com"
	data[string(semconv.EnduserIDKey)] = string(user.ID)

	event, _, err := service.plunkClient.Tracker.TrackEvent(ctx, &plunk.TrackEventRequest{
		Email:      user.Email,
		Event:      "contact.created",
		Subscribed: true,
		Data:       data,
	})
	if err != nil {
		msg := fmt.Sprintf("cannot create contact for user with id [%s]", userID)
		return service.tracer.WrapErrorSpan(span, stacktrace.Propagate(err, msg))
	}

	ctxLogger.Info(fmt.Sprintf("user [%s] added to marketting list with contact ID [%s] and event ID [%s]", userID, event.Data.Contact, event.Data.Event))
	return nil
}
