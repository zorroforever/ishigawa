package connect

import (
	"context"
	"errors"

	"github.com/go-kit/kit/endpoint"
	"github.com/micromdm/mdm"
)

type mdmConnectRequest struct {
	mdm.Response
}

type mdmConnectResponse struct {
	payload []byte
	Err     error `plist:"error,omitempty"`
}

func (r mdmConnectResponse) error() error { return r.Err }

// errInvalidMessageType is an invalid checking command.
var errInvalidMessageType = errors.New("invalid message type")

type Endpoints struct {
	ConnectEndpoint endpoint.Endpoint
}

func MakeConnectEndpoint(svc ConnectService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(mdmConnectRequest)
		if req.UserID != nil {
			// don't handle user
			return mdmConnectResponse{}, nil
		}
		payload, err := svc.Acknowledge(ctx, req.Response)
		return mdmConnectResponse{payload, err}, nil
	}
}
