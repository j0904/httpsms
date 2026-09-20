package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NdoleStudio/httpsms/pkg/events"
	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/NdoleStudio/httpsms/pkg/entities"
	"github.com/NdoleStudio/httpsms/pkg/repositories"
	"github.com/NdoleStudio/httpsms/pkg/telemetry"
	"github.com/google/uuid"
	"github.com/palantir/stacktrace"
	"go.opentelemetry.io/otel/trace"
)

// PhoneNotificationService sends out notifications to mobile phones.
// Phones without push support poll for outstanding messages instead, so no
// push client is needed.
type PhoneNotificationService struct {
	service
	logger                        telemetry.Logger
	tracer                        telemetry.Tracer
	phoneNotificationRepository   repositories.PhoneNotificationRepository
	phoneRepository               repositories.PhoneRepository
	messageSendScheduleRepository repositories.MessageSendScheduleRepository
	eventDispatcher               *EventDispatcher
}

// NewNotificationService creates a new PhoneNotificationService
func NewNotificationService(
	logger telemetry.Logger,
	tracer telemetry.Tracer,
	phoneRepository repositories.PhoneRepository,
	phoneNotificationRepository repositories.PhoneNotificationRepository,
	messageSendScheduleRepository repositories.MessageSendScheduleRepository,
	dispatcher *EventDispatcher,
) (s *PhoneNotificationService) {
	return &PhoneNotificationService{
		logger:                      logger.WithService(fmt.Sprintf("%T", &PhoneNotificationService{})),
		tracer:                      tracer,
		phoneNotificationRepository: phoneNotificationRepository,
		phoneRepository:             phoneRepository,
		messageSendScheduleRepository: messageSendScheduleRepository,
		eventDispatcher:               dispatcher,
	}
}

// DeleteAllForUser deletes all entities.PhoneNotification for an entities.UserID.
func (service *PhoneNotificationService) DeleteAllForUser(ctx context.Context, userID entities.UserID) error {
	ctx, span, ctxLogger := service.tracer.StartWithLogger(ctx, service.logger)
	defer span.End()

	if err := service.phoneNotificationRepository.DeleteAllForUser(ctx, userID); err != nil {
		msg := fmt.Sprintf("could not delete all [entities.PhoneNotification] for user with ID [%s]", userID)
		return service.tracer.WrapErrorSpan(span, stacktrace.Propagate(err, msg))
	}

	ctxLogger.Info(fmt.Sprintf("deleted all [entities.PhoneNotification] for user with ID [%s]", userID))
	return nil
}

// DeleteByMessageID deletes all entities.PhoneNotification for a user and message ID.
func (service *PhoneNotificationService) DeleteByMessageID(ctx context.Context, userID entities.UserID, messageID uuid.UUID) error {
	ctx, span, ctxLogger := service.tracer.StartWithLogger(ctx, service.logger)
	defer span.End()

	if err := service.phoneNotificationRepository.DeleteByMessageID(ctx, userID, messageID); err != nil {
		msg := fmt.Sprintf("could not delete [entities.PhoneNotification] for user [%s] and message with ID [%s]", userID, messageID)
		return service.tracer.WrapErrorSpan(span, stacktrace.Propagate(err, msg))
	}

	ctxLogger.Info(fmt.Sprintf("deleted [entities.PhoneNotification] for user [%s] and message with ID [%s]", userID, messageID))
	return nil
}

// SendHeartbeatFCM sends a heartbeat message so the phone can request a heartbeat
func (service *PhoneNotificationService) SendHeartbeatFCM(ctx context.Context, payload *events.PhoneHeartbeatMissedPayload) error {
	ctx, span, ctxLogger := service.tracer.StartWithLogger(ctx, service.logger)
	defer span.End()

	phone, err := service.phoneRepository.LoadByID(ctx, payload.UserID, payload.PhoneID)
	if err != nil {
		msg := fmt.Sprintf("cannot load phone with userID [%s] and phoneID [%s]", payload.UserID, payload.PhoneID)
		return service.tracer.WrapErrorSpan(span, stacktrace.Propagate(err, msg))
	}

	// Phones poll for heartbeats and outstanding messages; there is no push.
	ctxLogger.Info(fmt.Sprintf("skipping heartbeat push for phone with ID [%s], phone polls instead", phone.ID))
	return nil
}

// PhoneNotificationSendParams are parameters for sending a notification
type PhoneNotificationSendParams struct {
	UserID              entities.UserID
	PhoneID             uuid.UUID
	PhoneNotificationID uuid.UUID
	Source              string
	ScheduledAt         time.Time
	MessageID           uuid.UUID
}

// Send sends a message when a message is sent
func (service *PhoneNotificationService) Send(ctx context.Context, params *PhoneNotificationSendParams) error {
	ctx, span, ctxLogger := service.tracer.StartWithLogger(ctx, service.logger)
	defer span.End()

	phone, err := service.phoneRepository.LoadByID(ctx, params.UserID, params.PhoneID)
	if err != nil {
		msg := fmt.Sprintf("cannot load phone with userID [%s] and phoneID [%s]", params.UserID, params.PhoneID)
		return service.handleNotificationFailed(ctx, errors.New(msg), params)
	}

	// Phones poll for outstanding messages; the notification is handed off
	// here and the phone claims each message with GetOutstanding.
	ctxLogger.Info(fmt.Sprintf("leaving notification [%s] for message [%s] for phone [%s] to poll", params.PhoneNotificationID, params.MessageID, phone.ID))
	return service.handleNotificationSent(ctx, phone, "poll", params)
}

// PhoneNotificationScheduleParams are parameters for sending a notification
type PhoneNotificationScheduleParams struct {
	UserID            entities.UserID
	Owner             string
	Source            string
	Encrypted         bool
	Contact           string
	Content           string
	SIM               entities.SIM
	MessageID         uuid.UUID
	ExactSendTime     bool
	ScheduledSendTime *time.Time
}

// Schedule a notification to be sent to a phone
func (service *PhoneNotificationService) Schedule(ctx context.Context, params *PhoneNotificationScheduleParams) error {
	ctx, span := service.tracer.Start(ctx)
	defer span.End()

	ctxLogger := service.tracer.CtxLogger(service.logger, span)

	phone, err := service.phoneRepository.Load(ctx, params.UserID, params.Owner)
	if err != nil {
		msg := fmt.Sprintf("cannot load phone with userID [%s] and phone [%s]", params.UserID, params.Owner)
		return service.tracer.WrapErrorSpan(span, stacktrace.Propagate(err, msg))
	}

	notification := &entities.PhoneNotification{
		ID:          uuid.New(),
		MessageID:   params.MessageID,
		UserID:      params.UserID,
		PhoneID:     phone.ID,
		Status:      entities.PhoneNotificationStatusPending,
		ScheduledAt: time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if params.ExactSendTime && params.ScheduledSendTime != nil {
		return service.scheduleExact(ctx, span, ctxLogger, params, phone, notification)
	}

	var schedule *entities.MessageSendSchedule
	if phone.MessageSendScheduleID != nil {
		schedule, err = service.messageSendScheduleRepository.Load(ctx, params.UserID, *phone.MessageSendScheduleID)
		if err != nil && stacktrace.GetCode(err) != repositories.ErrCodeNotFound {
			msg := fmt.Sprintf("cannot load send schedule [%s] for phone [%s]", *phone.MessageSendScheduleID, phone.ID)
			return service.tracer.WrapErrorSpan(span, stacktrace.Propagate(err, msg))
		}
	}

	if schedule != nil {
		ctxLogger.Info(fmt.Sprintf("loaded [%T] with ID [%s] for phone [%s]", schedule, schedule.ID, phone.ID))
	}

	if err = service.phoneNotificationRepository.Schedule(ctx, phone.MessagesPerMinute, schedule, notification); err != nil {
		msg := fmt.Sprintf("cannot schedule notification for message [%s] to phone [%s]", params.MessageID, phone.ID)
		return service.tracer.WrapErrorSpan(span, stacktrace.Propagate(err, msg))
	}

	if err = service.dispatchMessageNotificationScheduled(ctx, params, notification); err != nil {
		ctxLogger.Error(err)
	}

	if err = service.dispatchMessageNotificationSend(ctx, params.Source, notification); err != nil {
		return service.tracer.WrapErrorSpan(span, err)
	}

	ctxLogger.Info(fmt.Sprintf(
		"message with id [%s] notification scheduled for [%s] with id [%s] with phone schedule ID [%s]",
		params.MessageID,
		notification.ScheduledAt,
		notification.ID,
		phone.MessageSendScheduleID,
	))
	return nil
}

func (service *PhoneNotificationService) scheduleExact(
	ctx context.Context,
	span trace.Span,
	ctxLogger telemetry.Logger,
	params *PhoneNotificationScheduleParams,
	phone *entities.Phone,
	notification *entities.PhoneNotification,
) error {
	scheduledAt := *params.ScheduledSendTime
	if scheduledAt.Before(time.Now().UTC()) {
		scheduledAt = time.Now().UTC()
	}
	notification.ScheduledAt = scheduledAt

	if err := service.phoneNotificationRepository.ScheduleExact(ctx, notification); err != nil {
		msg := fmt.Sprintf("cannot schedule exact notification for message [%s] to phone [%s]", params.MessageID, phone.ID)
		return service.tracer.WrapErrorSpan(span, stacktrace.Propagate(err, msg))
	}

	if err := service.dispatchMessageNotificationScheduled(ctx, params, notification); err != nil {
		ctxLogger.Error(err)
	}

	if err := service.dispatchMessageNotificationSend(ctx, params.Source, notification); err != nil {
		return service.tracer.WrapErrorSpan(span, err)
	}

	ctxLogger.Info(fmt.Sprintf(
		"message with id [%s] exact notification scheduled for [%s] with id [%s]",
		params.MessageID,
		notification.ScheduledAt,
		notification.ID,
	))
	return nil
}

func (service *PhoneNotificationService) dispatchMessageNotificationSend(
	ctx context.Context,
	source string,
	notification *entities.PhoneNotification,
) error {
	event, err := service.createMessageNotificationSendEvent(source, &events.MessageNotificationSendPayload{
		MessageID:      notification.MessageID,
		UserID:         notification.UserID,
		PhoneID:        notification.PhoneID,
		ScheduledAt:    notification.ScheduledAt,
		NotificationID: notification.ID,
	})
	if err != nil {
		return stacktrace.Propagate(err, fmt.Sprintf("cannot create [%s] event for notification [%s]", events.EventTypeMessageNotificationSend, notification.ID))
	}

	if _, err = service.eventDispatcher.DispatchWithTimeout(ctx, event, notification.ScheduledAt.Sub(time.Now())); err != nil {
		return stacktrace.Propagate(err, fmt.Sprintf("cannot dispatch event [%s] for notification [%s]", event.Type(), notification.ID))
	}
	return nil
}

func (service *PhoneNotificationService) dispatchMessageNotificationScheduled(
	ctx context.Context,
	params *PhoneNotificationScheduleParams,
	notification *entities.PhoneNotification,
) error {
	event, err := service.createMessageNotificationScheduledEvent(params.Source, &events.MessageNotificationScheduledPayload{
		MessageID:      notification.MessageID,
		Owner:          params.Owner,
		Contact:        params.Contact,
		Encrypted:      params.Encrypted,
		Content:        params.Content,
		SIM:            params.SIM,
		UserID:         notification.UserID,
		PhoneID:        notification.PhoneID,
		ScheduledAt:    notification.ScheduledAt,
		NotificationID: notification.ID,
	})
	if err != nil {
		return stacktrace.Propagate(err, fmt.Sprintf("cannot create [%s] event for notification [%s]", events.EventTypeMessageNotificationScheduled, notification.ID))
	}

	if err = service.eventDispatcher.Dispatch(ctx, event); err != nil {
		return stacktrace.Propagate(err, fmt.Sprintf("cannot dispatch event [%s] for notification [%s]", event.Type(), notification.ID))
	}
	return nil
}

func (service *PhoneNotificationService) handleNotificationFailed(ctx context.Context, err error, params *PhoneNotificationSendParams) error {
	ctx, span := service.tracer.Start(ctx)
	defer span.End()

	ctxLogger := service.tracer.CtxLogger(service.logger, span)

	msg := fmt.Sprintf("cannot send notification for message [%s] to phone [%s]", params.MessageID, params.PhoneNotificationID)
	ctxLogger.Warn(stacktrace.Propagate(err, msg))

	event, err := service.createMessageNotificationFailedEvent(params.Source, err.Error(), params)
	if err != nil {
		return stacktrace.Propagate(err, fmt.Sprintf("cannot create [%s] event for notification [%s]", events.EventTypeMessageNotificationFailed, params.PhoneNotificationID))
	}

	if err = service.eventDispatcher.Dispatch(ctx, event); err != nil {
		return stacktrace.Propagate(err, fmt.Sprintf("cannot dispatch event [%s] for notification [%s]", event.Type(), params.PhoneNotificationID))
	}

	service.updateStatus(ctx, params.PhoneNotificationID, entities.PhoneNotificationStatusFailed)
	return nil
}

func (service *PhoneNotificationService) handleNotificationSent(
	ctx context.Context,
	phone *entities.Phone,
	result string,
	params *PhoneNotificationSendParams,
) error {
	ctx, span := service.tracer.Start(ctx)
	defer span.End()

	ctxLogger := service.tracer.CtxLogger(service.logger, span)

	ctxLogger.Info(fmt.Sprintf("sent notification [%s] for message [%s] to phone [%s]", result, params.MessageID, params.PhoneID))

	event, err := service.createMessageNotificationSentEvent(params.Source, phone, result, params)
	if err != nil {
		return stacktrace.Propagate(err, fmt.Sprintf("cannot create [%s] event for notification [%s]", events.EventTypeMessageNotificationSent, params.PhoneNotificationID))
	}

	if err = service.eventDispatcher.Dispatch(ctx, event); err != nil {
		return stacktrace.Propagate(err, fmt.Sprintf("cannot dispatch event [%s] for notification [%s]", event.Type(), params.PhoneNotificationID))
	}

	service.updateStatus(ctx, params.PhoneNotificationID, entities.PhoneNotificationStatusSent)
	return nil
}

func (service *PhoneNotificationService) createMessageNotificationScheduledEvent(
	source string,
	payload *events.MessageNotificationScheduledPayload,
) (cloudevents.Event, error) {
	return service.createEvent(events.EventTypeMessageNotificationScheduled, source, payload)
}

func (service *PhoneNotificationService) createMessageNotificationSendEvent(
	source string,
	payload *events.MessageNotificationSendPayload,
) (cloudevents.Event, error) {
	return service.createEvent(events.EventTypeMessageNotificationSend, source, payload)
}

func (service *PhoneNotificationService) createMessageNotificationSentEvent(
	source string,
	phone *entities.Phone,
	fcmMessageID string,
	params *PhoneNotificationSendParams,
) (cloudevents.Event, error) {
	event := cloudevents.NewEvent()

	event.SetSource(source)
	event.SetType(events.EventTypeMessageNotificationSent)
	event.SetTime(time.Now().UTC())
	event.SetID(uuid.New().String())

	payload := events.MessageNotificationSentPayload{
		MessageID:                 params.MessageID,
		UserID:                    params.UserID,
		PhoneID:                   params.PhoneID,
		ScheduledAt:               params.ScheduledAt,
		MessageExpirationDuration: phone.MessageExpirationDuration(),
		FcmMessageID:              fcmMessageID,
		NotificationSentAt:        time.Now().UTC(),
		NotificationID:            params.PhoneNotificationID,
	}

	if err := event.SetData(cloudevents.ApplicationJSON, payload); err != nil {
		msg := fmt.Sprintf("cannot encode %T [%#+v] as JSON", payload, payload)
		return event, stacktrace.Propagate(err, msg)
	}

	return event, nil
}

func (service *PhoneNotificationService) createMessageNotificationFailedEvent(
	source string,
	errorMessage string,
	params *PhoneNotificationSendParams,
) (cloudevents.Event, error) {
	event := cloudevents.NewEvent()

	event.SetSource(source)
	event.SetType(events.EventTypeMessageNotificationFailed)
	event.SetTime(time.Now().UTC())
	event.SetID(uuid.New().String())

	payload := events.MessageNotificationFailedPayload{
		MessageID:            params.MessageID,
		UserID:               params.UserID,
		PhoneID:              params.PhoneID,
		ErrorMessage:         errorMessage,
		NotificationFailedAt: time.Now().UTC(),
		NotificationID:       params.PhoneNotificationID,
	}

	if err := event.SetData(cloudevents.ApplicationJSON, payload); err != nil {
		msg := fmt.Sprintf("cannot encode %T [%#+v] as JSON", payload, payload)
		return event, stacktrace.Propagate(err, msg)
	}

	return event, nil
}

func (service *PhoneNotificationService) updateStatus(
	ctx context.Context,
	notificationID uuid.UUID,
	status entities.PhoneNotificationStatus,
) {
	ctx, span := service.tracer.Start(ctx)
	defer span.End()

	ctxLogger := service.tracer.CtxLogger(service.logger, span)

	err := service.phoneNotificationRepository.UpdateStatus(ctx, notificationID, status)
	if err != nil {
		msg := fmt.Sprintf("cannot update status of notification with id [%s] to [%s]", notificationID, status)
		ctxLogger.Error(stacktrace.Propagate(err, msg))
	}

	ctxLogger.Info(fmt.Sprintf("updated status of notification with id [%s] to [%s]", notificationID, status))
}
