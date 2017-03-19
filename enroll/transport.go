package enroll

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/fullsailor/pkcs7"
	"github.com/go-kit/kit/log"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/groob/plist"
)

// ServiceHandler returns an HTTP Handler for the enroll service
func ServiceHandler(ctx context.Context, svc Service, logger log.Logger) http.Handler {
	r := mux.NewRouter()
	e := MakeServerEndpoints(svc)
	opts := []httptransport.ServerOption{
		httptransport.ServerErrorLogger(logger),
	}
	r.Methods("GET", "POST").Path("/mdm/enroll").Handler(httptransport.NewServer(
		e.GetEnrollEndpoint,
		decodeMDMEnrollRequest,
		encodeResponse,
		opts...,
	))

	return r
}

func decodeMDMEnrollRequest(_ context.Context, r *http.Request) (interface{}, error) {
	switch r.Method {
	case "GET":
		return mdmEnrollRequest{}, nil
	case "POST":
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		p7, err := pkcs7.Parse(data)
		if err != nil {
			return nil, err
		}
		// TODO: We should verify but not currently possible. Apple
		// does no provide a cert for the CA.
		var request depEnrollmentRequest
		if err := plist.Unmarshal(p7.Content, &request); err != nil {
			return nil, err
		}
		return request, nil
	default:
		return nil, errors.New("unknown enrollment method")
	}
}

func encodeResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(mdmEnrollResponse)

	w.Header().Set("Content-Type", "application/x-apple-aspen-config")

	if err := plist.NewEncoder(w).Encode(resp); err != nil {
		return err
	}

	return nil
}
