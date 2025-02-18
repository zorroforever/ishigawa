package dep

import (
	"context"
	"errors"
	"github.com/go-kit/kit/endpoint"
	"github.com/micromdm/micromdm/dep"
	"github.com/micromdm/micromdm/pkg/httputil"
	"net/http"
)

type DisownDevicesEndpoint struct{}
type DisownDevicesRequest struct {
	Serial string `json:"serial"`
}

func (svc *DEPService) DisownDevices(ctx context.Context, r *dep.DisownDevicesRequest) (*dep.DisownDevicesResponse, error) {
	if svc.client == nil {
		return nil, errors.New("DEP not configured yet. add a DEP token to enable DEP")
	}
	return svc.client.DisownDevices(r)

}

type disownDevicesRequest struct {
	*dep.DisownDevicesRequest
}

type disownDevicesResponse struct {
	*dep.DisownDevicesResponse
	Err error `json:"err,omitempty"`
}

func (r disownDevicesResponse) Failed() error { return r.Err }

func decodeDisownDevicesRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var req disownDevicesRequest
	err := httputil.DecodeJSONRequest(r, &req)
	return req, err
}

func decodeDisownDevicesResponse(_ context.Context, r *http.Response) (interface{}, error) {
	var resp disownDevicesResponse
	err := httputil.DecodeJSONResponse(r, &resp)
	return resp, err
}

func MakeDisownDevicesEndpoint(svc Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(disownDevicesRequest)
		resp, err := svc.DisownDevices(ctx, req.DisownDevicesRequest)
		return disownDevicesResponse{
			DisownDevicesResponse: resp,
			Err:                   err,
		}, nil
	}
}

func (e Endpoints) DisownDevices(ctx context.Context, r *dep.DisownDevicesRequest) (*dep.DisownDevicesResponse, error) {
	request := r
	resp, err := e.DoDisownDevicesEndpoint(ctx, request)
	if err != nil {
		return nil, err
	}
	response := resp.(disownDevicesResponse)
	return response.DisownDevicesResponse, err
}
