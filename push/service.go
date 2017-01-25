package push

import (
	"encoding/json"

	"github.com/RobotsAndPencils/buford/payload"
	"github.com/RobotsAndPencils/buford/push"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type Push struct {
	db      *DB
	pushsvc *push.Service
}

func New(db *DB, push *push.Service) *Push {
	return &Push{db, push}
}

func (svc *Push) Push(ctx context.Context, deviceUDID string) (string, error) {
	info, err := svc.db.PushInfo(deviceUDID)
	if err != nil {
		return "", errors.Wrap(err, "retrieving PushInfo by UDID")
	}

	p := payload.MDM{Token: info.PushMagic}
	valid := push.IsDeviceTokenValid(info.Token)
	if !valid {
		return "", errors.New("invalid push token")
	}
	jsonPayload, err := json.Marshal(p)
	if err != nil {
		return "", errors.Wrap(err, "marshalling push notification payload")
	}
	return svc.pushsvc.Push(info.Token, nil, jsonPayload)
}
