package connect

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/micromdm/mdm"
)

// The ConnectService accepts responses sent to an MDM server by an enrolled
// device.
type ConnectService interface {

	// Acknowledge acknowledges a response sent by a device and returns
	// the next payload if one is available.
	Acknowledge(ctx context.Context, req mdm.Response) (payload []byte, err error)
}

type connectSvc struct {
	queue *Queue
}

func New(queue *Queue) (ConnectService, error) {
	return &connectSvc{
		queue: queue,
	}, nil
}

func (svc *connectSvc) Acknowledge(ctx context.Context, req mdm.Response) (payload []byte, err error) {
	fmt.Printf("connected udid=%s type=%s, status=%s\n", req.UDID, req.RequestType, req.Status)
	return nil, nil
}
