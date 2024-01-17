package dep

import (
	"context"
	"errors"
	"github.com/go-kit/kit/endpoint"
	"github.com/micromdm/micromdm/dep"
	"github.com/micromdm/micromdm/pkg/httputil"
	"net/http"
)

func (svc *DEPService) ActivationLock(ctx context.Context, device string, escrowKey string, lostMessage string) (*dep.ActivationLockResponse, error) {
	if svc.client == nil {
		return nil, errors.New("DEP not configured yet. add a DEP token to enable DEP")
	}
	var req *dep.ActivationLockRequest

	return svc.client.ActivationLock(device, escrowKey, lostMessage)
}

type activationLockRequest struct {
	*dep.ActivationLockRequest
}

type activationLockResponse struct {
	*dep.ActivationLockResponse
	Err error `json:"err,omitempty"`
}

func (r activationLockResponse) Failed() error { return r.Err }

func decodeActivationLockRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var req activationLockRequest
	err := httputil.DecodeJSONRequest(r, &req)
	return req, err
}

func decodeActivationLockResponse(_ context.Context, r *http.Response) (interface{}, error) {
	var resp activationLockResponse
	err := httputil.DecodeJSONResponse(r, &resp)
	return resp, err
}

func MakeDoActivationLockEndpoint(svc Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(activationLockRequest)
		resp, err := svc.ActivationLock(ctx, req.Device, req.EscrowKey, req.LostMessage)
		return activationLockResponse{
			ActivationLockResponse: resp,
			Err:                    err,
		}, nil
	}
}

func (e Endpoints) DoActivationLock(ctx context.Context, device string, escrowKey string, lostMessage string) (*dep.ActivationLockResponse, error) {
	request := activationLockRequest{Device: device,
		EscrowKey:   escrowKey,
		LostMessage: lostMessage}
	resp, err := e.DoActivationLockEndpoint(ctx, request)
	if err != nil {
		return nil, err
	}
	response := resp.(activationLockResponse)
	return response.ActivationLockResponse, err
}
