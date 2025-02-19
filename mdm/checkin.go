package mdm

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/google/uuid"
	"github.com/micromdm/plist"
	"github.com/pkg/errors"
)

// BootstrapToken holds MDM Bootstrap Token data
type BootstrapToken struct {
	BootstrapToken []byte
}

type DeclarativeManagement interface {
	DeclarativeManagement(ctx context.Context, id, endpoint string, data []byte) ([]byte, error)
}

func (svc *MDMService) Checkin(ctx context.Context, event CheckinEvent) ([]byte, error) {
	// reject user settings at the loginwindow.
	// https://github.com/micromdm/micromdm/pull/379
	if event.Command.MessageType == "UserAuthenticate" {
		return nil, &rejectUserAuth{}
	}

	msg, err := MarshalCheckinEvent(&event)
	if err != nil {
		return nil, errors.Wrap(err, "marshal checkin event")
	}

	topic, err := topicFromMessage(event.Command.MessageType)
	if err != nil {
		return nil, errors.Wrap(err, "get checkin topic from message")
	}

	var resp []byte

	switch topic {
	case AuthenticateTopic:
		if err := svc.queue.Clear(ctx, event); err != nil {
			return nil, errors.Wrap(err, "clearing queue on enrollment attempt")
		}
	case GetBootstrapTokenTopic:
		udid := event.Command.UDID

		btBytes, err := svc.dev.GetBootstrapToken(ctx, udid)
		if err != nil {
			return nil, errors.Wrap(err, "fetching bootstrap token")
		}

		bt := &BootstrapToken{BootstrapToken: btBytes}

		resp, err = plist.Marshal(bt)
		if err != nil {
			return nil, errors.Wrap(err, "marshal bootstrap token")
		}
	case DeclarativeManagementTopic:
		if svc.dm == nil {
			return nil, stderrors.New("no Declarative Management handler")
		}

		udid := event.Command.UDID
		if event.Command.UserID != "" {
			udid = event.Command.UserID
		}
		if event.Command.EnrollmentID != "" {
			udid = event.Command.EnrollmentID
		}

		resp, err = svc.dm.DeclarativeManagement(ctx, udid, event.Command.Endpoint, event.Command.Data)
		if err != nil {
			return resp, fmt.Errorf("declarative management: %w", err)
		}
	}

	err = svc.pub.Publish(ctx, topic, msg)
	return resp, errors.Wrapf(err, "publish checkin on topic: %s", topic)
}

func topicFromMessage(messageType string) (string, error) {
	switch messageType {
	case "Authenticate":
		return AuthenticateTopic, nil
	case "TokenUpdate":
		return TokenUpdateTopic, nil
	case "CheckOut":
		return CheckoutTopic, nil
	case "GetBootstrapToken":
		return GetBootstrapTokenTopic, nil
	case "SetBootstrapToken":
		return SetBootstrapTokenTopic, nil
	case "DeclarativeManagement":
		return DeclarativeManagementTopic, nil
	default:
		return "", errors.Errorf("unknown checkin message type %s", messageType)
	}
}

type rejectUserAuth struct{}

func (e *rejectUserAuth) Error() string {
	return "reject user auth"
}
func (e *rejectUserAuth) UserAuthReject() bool {
	return true
}

func isRejectedUserAuth(err error) bool {
	type rejectUserAuthError interface {
		error
		UserAuthReject() bool
	}

	_, ok := errors.Cause(err).(rejectUserAuthError)
	return ok
}

type checkinRequest struct {
	Event CheckinEvent
}

type checkinResponse struct {
	Payload []byte
	Err     error `plist:"error,omitempty"`
}

func (r checkinResponse) Response() []byte { return r.Payload }
func (r checkinResponse) Failed() error    { return r.Err }

func decodeCheckinRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var cmd CheckinCommand
	body, err := mdmRequestBody(r, &cmd)
	if err != nil {
		return nil, errors.Wrap(err, "read MDM request")
	}

	values := r.URL.Query()
	params := make(map[string]string, len(values))
	for k, v := range values {
		params[k] = v[0]
	}

	event := CheckinEvent{
		ID:      uuid.New().String(),
		Time:    time.Now().UTC(),
		Command: cmd,
		Params:  params,
		Raw:     body,
	}
	req := checkinRequest{Event: event}
	return req, nil
}

func MakeCheckinEndpoint(svc Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(checkinRequest)
		payload, err := svc.Checkin(ctx, req.Event)
		return checkinResponse{Payload: payload, Err: err}, nil
	}
}
