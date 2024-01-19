package dep

import (
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"net/url"
	"os"
)

const (
	activationLockPath        = "device/activationlock"
	disableActivationLockPath = "deviceservicesworkers/escrowKeyUnlock"
)

type ActivationLockRequest struct {
	Device string `json:"device"`

	//  If the escrow key is not provided, the device will be locked to the person who created the MDM server in the portal.
	// https://developer.apple.com/documentation/devicemanagement/device_assignment/activation_lock_a_device/creating_and_using_bypass_codes
	// The EscrowKey is a hex-encoded PBKDF2 derivation of the bypass code. See activationlock.BypassCode.
	EscrowKey string `json:"escrow_key"`

	LostMessage string `json:"lost_message"`
}

type DisableActivationLockRequest struct {
	Serial      string `json:"serial"`
	Imei        string `json:"imei"`
	Imei2       string `json:"imei2"`
	Meid        string `json:"meid"`
	ProductType string `json:"productType"`
	OrgName     string `json:"orgName"`
	Guid        string `json:"guid"`
	EscrowKey   string `json:"escrowKey"`
}

type DisableActivationLockRequestBodyInfo struct {
	OrgName   string `json:"orgName"`
	Guid      string `json:"guid"`
	EscrowKey string `json:"escrowKey"`
}
type DisableActivationLockResponse struct {
}

type ActivationLockResponse struct {
	SerialNumber string `json:"serial_number"`
	Status       string `json:"response_status"`
}

func (c *Client) ActivationLock(alr *ActivationLockRequest) (*ActivationLockResponse, error) {
	req, err := c.newRequest("POST", activationLockPath, &alr)
	if err != nil {
		return nil, errors.Wrap(err, "create activation lock request")
	}

	var response ActivationLockResponse
	err = c.do(req, &response)
	return &response, errors.Wrap(err, "activation lock")
}

func (c *Client) DisableActivationLock(dalr *DisableActivationLockRequest) (*DisableActivationLockResponse, error) {
	logger := log.NewLogfmtLogger(os.Stderr)
	// 使用url.Values来拼接URL参数
	values := url.Values{}
	values.Add("serial", url.QueryEscape(dalr.Serial))
	values.Add("imei", url.QueryEscape(dalr.Imei))
	values.Add("imei2", url.QueryEscape(dalr.Imei2))
	values.Add("meid", url.QueryEscape(dalr.Meid))
	values.Add("productType", url.QueryEscape(dalr.ProductType))
	//values.Add("orgName", dalr.OrgName)
	//values.Add("guid", dalr.Guid)
	//values.Add("escrowKey", dalr.EscrowKey)
	body := DisableActivationLockRequestBodyInfo{
		OrgName:   url.QueryEscape(dalr.OrgName),
		Guid:      url.QueryEscape(dalr.Guid),
		EscrowKey: url.QueryEscape(dalr.EscrowKey),
	}
	level.Info(logger).Log(
		"msg", "DisableActivationLockRequestBodyInfo",
		"body.OrgName", url.QueryEscape(dalr.OrgName),
		"body.Guid", url.QueryEscape(dalr.Guid),
		"body.EscrowKey", url.QueryEscape(dalr.EscrowKey),
	)
	// 将url.Values编码为字符串形式
	queryString := values.Encode()
	var toUri = disableActivationLockPath + "?" + queryString
	level.Info(logger).Log(
		"msg", "DisableActivationLock body",
		"toUri", toUri,
		"body.OrgName", body.OrgName,
		"body.Guid", body.Guid,
		"body.EscrowKey", body.EscrowKey,
	)
	req, err := c.newRequest2("POST", toUri, &body)
	if err != nil {
		return nil, errors.Wrap(err, "create activation lock request")
	}

	var response DisableActivationLockResponse
	err = c.do2(req, &response)
	return &response, errors.Wrap(err, "activation lock")
}
