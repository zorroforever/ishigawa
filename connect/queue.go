package connect

import (
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/micromdm/nano/command"
	"github.com/micromdm/nano/pubsub"
	"github.com/pkg/errors"
)

const (
	DeviceCommandBucket = "mdm.DeviceCommands"
)

type Queue struct {
	*bolt.DB
}

func NewQueue(db *bolt.DB, sub pubsub.Subscriber) (*Queue, error) {
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(DeviceCommandBucket))
		return err
	})
	if err != nil {
		return nil, errors.Wrapf(err, "creating %s bucket", DeviceCommandBucket)
	}
	datastore := &Queue{
		DB: db,
	}
	if err := datastore.pollCommands(sub); err != nil {
		return nil, err
	}
	return datastore, nil
}

func (db *Queue) Save(cmd *DeviceCommand) error {
	tx, err := db.DB.Begin(true)
	if err != nil {
		return errors.Wrap(err, "begin transaction")
	}
	bkt := tx.Bucket([]byte(DeviceCommandBucket))
	if bkt == nil {
		return fmt.Errorf("bucket %q not found!", DeviceCommandBucket)
	}
	devproto, err := MarshalDeviceCommand(cmd)
	if err != nil {
		return errors.Wrap(err, "marshalling DeviceCommand")
	}
	key := []byte(cmd.DeviceUDID)
	if err := bkt.Put(key, devproto); err != nil {
		return errors.Wrap(err, "put DeviceCOmmand to boltdb")
	}
	return tx.Commit()
}

func (db *Queue) DeviceCommand(udid string) (*DeviceCommand, error) {
	var dev DeviceCommand
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DeviceCommandBucket))
		v := b.Get([]byte(udid))
		if v == nil {
			return &notFound{"DeviceCommand", fmt.Sprintf("udid %s", udid)}
		}
		return UnmarshalDeviceCommand(v, &dev)
	})
	if err != nil {
		return nil, err
	}
	return &dev, nil
}

type notFound struct {
	ResourceType string
	Message      string
}

func (e *notFound) Error() string {
	return fmt.Sprintf("not found: %s %s", e.ResourceType, e.Message)
}

func (db *Queue) pollCommands(sub pubsub.Subscriber) error {
	commandEvents, err := sub.Subscribe("command-queue", command.CommandTopic)
	if err != nil {
		return errors.Wrapf(err,
			"subscribing push to %s topic", command.CommandTopic)
	}
	go func() {
		for {
			select {
			case event := <-commandEvents:
				var ev command.Event
				if err := command.UnmarshalEvent(event.Message, &ev); err != nil {
					fmt.Println(err)
					continue
				}
				cmd, _ := db.DeviceCommand(ev.DeviceUDID)
				if cmd == nil {
					cmd = &DeviceCommand{
						DeviceUDID: ev.DeviceUDID,
						Commands: []Command{{
							UUID:    ev.Payload.CommandUUID,
							Payload: nil, // TODO
						}},
					}
				} else {
					cmd.Commands = append(cmd.Commands, Command{
						UUID:    ev.Payload.CommandUUID,
						Payload: nil, // TODO
					})
				}

				fmt.Printf("queued event for device: %s\n", ev.DeviceUDID)
			}
		}
	}()

	return nil
}

func isNotFound(err error) bool {
	if _, ok := err.(*notFound); ok {
		return true
	}
	return false
}
