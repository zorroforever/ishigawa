package dep

import (
	"context"
	"errors"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/micromdm/micromdm/dep"
	"github.com/micromdm/micromdm/pkg/activationlock"
	"github.com/micromdm/micromdm/pkg/httputil"
	"net/http"
	"os"
)

func (svc *DEPService) ActivationLock(ctx context.Context, r *dep.ActivationLockRequest) (*dep.ActivationLockResponse, error) {
	if svc.client == nil {
		return nil, errors.New("DEP not configured yet. add a DEP token to enable DEP")
	}
	return svc.client.ActivationLock(r)

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
		logger := log.NewLogfmtLogger(os.Stderr)
		req := request.(activationLockRequest)
		var orgKey = req.ActivationLockRequest.EscrowKey
		level.Info(logger).Log(
			"msg", "MakeDoActivationLockEndpoint",
			"escrow key org string", &orgKey,
		)
		if orgKey != "" {
			bypassCode, err := activationlock.Create(nil)
			var hashReq = dep.ActivationLockRequest{
				Device:      req.ActivationLockRequest.Device,
				LostMessage: req.ActivationLockRequest.LostMessage,
				EscrowKey:   bypassCode.Hash(),
			}
			level.Info(logger).Log(
				"msg", "MakeDoActivationLockEndpoint",
				"escrow key hash", (&bypassCode).Hash(),
			)
			level.Info(logger).Log(
				"msg", "MakeDoActivationLockEndpoint",
				"escrow key string", (&bypassCode).String(),
			)
			resp, err := svc.ActivationLock(ctx, &hashReq)
			return activationLockResponse{
				ActivationLockResponse: resp,
				Err:                    err,
			}, nil
		} else {
			resp, err := svc.ActivationLock(ctx, req.ActivationLockRequest)
			return activationLockResponse{
				ActivationLockResponse: resp,
				Err:                    err,
			}, nil
		}
	}
}

func (e Endpoints) ActivationLock(ctx context.Context, r *dep.ActivationLockRequest) (*dep.ActivationLockResponse, error) {
	request := r
	resp, err := e.DoActivationLockEndpoint(ctx, request)
	if err != nil {
		return nil, err
	}
	response := resp.(activationLockResponse)
	return response.ActivationLockResponse, err
}
