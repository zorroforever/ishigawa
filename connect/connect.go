package connect

import (
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
