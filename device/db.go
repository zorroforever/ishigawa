package device

import (
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/micromdm/nano/checkin"
	"github.com/micromdm/nano/pubsub"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

const DeviceBucket = "mdm.Devices"

type DB struct {
	*bolt.DB
}

func NewDB(db *bolt.DB, sub pubsub.Subscriber) (*DB, error) {
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(DeviceBucket))
		return err
	})
	if err != nil {
		return nil, errors.Wrapf(err, "creating %s bucket", DeviceBucket)
	}
	datastore := &DB{
		DB: db,
	}
	if err := datastore.pollCheckin(sub); err != nil {
		return nil, err
	}
	return datastore, nil
}

func (db *DB) Save(dev *Device) error {
	tx, err := db.DB.Begin(true)
	if err != nil {
		return errors.Wrap(err, "begin transaction")
	}
	bkt := tx.Bucket([]byte(DeviceBucket))
	if bkt == nil {
		return fmt.Errorf("bucket %q not found!", DeviceBucket)
	}
	devproto, err := MarshalDevice(dev)
	if err != nil {
		return errors.Wrap(err, "marshalling device")
	}
	indexes := []string{dev.UDID, dev.UUID}
	for _, idx := range indexes {
		if idx == "" {
			continue
		}
		key := []byte(idx)
		if err := bkt.Put(key, devproto); err != nil {
			return errors.Wrap(err, "put device to boltdb")
		}
	}
	return tx.Commit()
}

type notFound struct {
	ResourceType string
	Message      string
}

func (e *notFound) Error() string {
	return fmt.Sprintf("not found: %s %s", e.ResourceType, e.Message)
}

func (db *DB) DeviceByUDID(udid string) (*Device, error) {
	var dev Device
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DeviceBucket))
		v := b.Get([]byte(udid))
		if v == nil {
			return &notFound{"Device", fmt.Sprintf("udid %s", udid)}
		}
		return UnmarshalDevice(v, &dev)
	})
	if err != nil {
		return nil, err
	}
	return &dev, nil
}

func isNotFound(err error) bool {
	if _, ok := err.(*notFound); ok {
		return true
	}
	return false
}

func (db *DB) pollCheckin(sub pubsub.Subscriber) error {
	authenticateEvents, err := sub.Subscribe("devices", checkin.AuthenticateTopic)
	if err != nil {
		return errors.Wrapf(err,
			"subscribing devices to %s topic", checkin.AuthenticateTopic)
	}
	tokenUpdateEvents, err := sub.Subscribe("devices", checkin.TokenUpdateTopic)
	if err != nil {
		return errors.Wrapf(err,
			"subscribing devices to %s topic", checkin.TokenUpdateTopic)
	}
	checkoutEvents, err := sub.Subscribe("devices", checkin.CheckoutTopic)
	if err != nil {
		return errors.Wrapf(err,
			"subscribing devices to %s topic", checkin.CheckoutTopic)
	}
	go func() {
		for {
			select {
			case event := <-authenticateEvents:
				var ev checkin.Event
				if err := checkin.UnmarshalEvent(event.Message, &ev); err != nil {
					fmt.Println(err)
					continue
				}
				_, err = db.DeviceByUDID(ev.Command.UDID)
				if err != nil {
					if isNotFound(err) {
						if err := db.Save(&Device{
							UUID:         uuid.NewV4().String(),
							UDID:         ev.Command.UDID,
							OSVersion:    ev.Command.OSVersion,
							BuildVersion: ev.Command.BuildVersion,
							ProductName:  ev.Command.ProductName,
							SerialNumber: ev.Command.SerialNumber,
							IMEI:         ev.Command.IMEI,
							MEID:         ev.Command.MEID,
							DeviceName:   ev.Command.DeviceName,
							// Challenge:    ev.Command.Challenge,
							Model:     ev.Command.Model,
							ModelName: ev.Command.ModelName,
						}); err != nil {
							fmt.Println(err)
							continue
						}
						continue
					}
					fmt.Println(err)
					continue
				}
			case event := <-tokenUpdateEvents:
				var ev checkin.Event
				if err := checkin.UnmarshalEvent(event.Message, &ev); err != nil {
					fmt.Println(err)
					continue
				}
				if ev.Command.UserID != "" {
					continue
				}
				dev, err := db.DeviceByUDID(ev.Command.UDID)
				if err != nil {
					fmt.Println(err)
					continue
				}
				dev.Token = ev.Command.Token.String()
				dev.PushMagic = ev.Command.PushMagic
				dev.UnlockToken = ev.Command.UnlockToken.String()
				dev.AwaitingConfiguration = ev.Command.AwaitingConfiguration
				dev.Enrolled = true
				if err := db.Save(dev); err != nil {
					fmt.Println(err)
					continue
				}
			case event := <-checkoutEvents:
				var ev checkin.Event
				if err := checkin.UnmarshalEvent(event.Message, &ev); err != nil {
					fmt.Println(err)
				}
				dev, err := db.DeviceByUDID(ev.Command.UDID)
				if err != nil {
					fmt.Println(err)
					continue
				}
				dev.Enrolled = false
				if err := db.Save(dev); err != nil {
					fmt.Println(err)
					continue
				}
			}
		}
	}()

	return nil
}
