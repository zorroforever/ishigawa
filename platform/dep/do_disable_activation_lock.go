package dep

import (
	"context"
	"errors"
	"github.com/go-kit/kit/endpoint"
	"github.com/micromdm/micromdm/dep"
	"github.com/micromdm/micromdm/pkg/httputil"
	"net/http"
)

func (svc *DEPService) DisableActivationLock(ctx context.Context, r *dep.DisableActivationLockRequest) (*dep.DisableActivationLockResponse, error) {
	if svc.client2 == nil {
		return nil, errors.New("DEP not configured yet. add a DEP token to enable DEP")
	}
	return svc.client2.DisableActivationLock(r)

}

type disableActivationLockRequest struct {
	*dep.DisableActivationLockRequest
}

type disableActivationLockResponse struct {
	*dep.DisableActivationLockResponse
	Err error `json:"err,omitempty"`
}

func (r disableActivationLockResponse) Failed() error { return r.Err }

func decodeDisableActivationLockRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var req disableActivationLockRequest
	err := httputil.DecodeJSONRequest(r, &req)
	return req, err
}

func decodeDisableActivationLockResponse(_ context.Context, r *http.Response) (interface{}, error) {
	var resp disableActivationLockResponse
	err := httputil.DecodeJSONResponse(r, &resp)
	return resp, err
}

func MakeDisableActivationLockEndpoint(svc Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(disableActivationLockRequest)
		resp, err := svc.DisableActivationLock(ctx, req.DisableActivationLockRequest)
		return disableActivationLockResponse{
			DisableActivationLockResponse: resp,
			Err:                           err,
		}, nil
	}
}

func (e Endpoints) DisableActivationLock(ctx context.Context, r *dep.DisableActivationLockRequest) (*dep.DisableActivationLockResponse, error) {
	request := r
	resp, err := e.DoDisableActivationLockEndpoint(ctx, request)
	if err != nil {
		return nil, err
	}
	response := resp.(disableActivationLockResponse)
	return response.DisableActivationLockResponse, err
}
