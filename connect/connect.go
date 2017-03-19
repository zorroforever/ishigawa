package connect

import (
	"fmt"
	"log"

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
	dc, err := svc.queue.DeviceCommand(req.UDID)
	if err != nil {
		log.Println(err)
		return nil, nil
	}

	if len(dc.Commands) == 0 {
		return nil, nil
	}
	payload = dc.Commands[0].Payload

	// delete first element
	dc.Commands = append(dc.Commands[:0], dc.Commands[0+1:]...)

	if err := svc.queue.Save(dc); err != nil {
		return nil, err
	}

	return payload, nil
}
