package dep

import (
	"github.com/pkg/errors"
	"net/url"
)

const (
	disownDevicesPath        = "device/disown"
)

type DisownDevicesRequest struct {
	Device []string `json:"device"`

}

type DisownDevicesResponse struct {
	Status string `json:"response_status"`
}


func (c *Client) DisownDevices(alr *DisownDevicesRequest) (*DisownDevicesResponse, error) {
	req, err := c.newRequest("POST", disownDevicesPath, &alr)
	if err != nil {
		return nil, errors.Wrap(err, "create disown device lock request")
	}
	var response DisownDevicesResponse
	err = c.do(req, &response)
	return &response, errors.Wrap(err, "disown device")
}

