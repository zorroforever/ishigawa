package depsync

import (
	"fmt"
	"log"
	"time"

	"github.com/micromdm/dep"
	"github.com/micromdm/micromdm/pubsub"
)

const (
	SyncTopic = "mdm.DepSync"
)

type Syncer interface {
	privateDEPSyncer() bool
}

type watcher struct {
	initial   bool
	config    *dep.Config
	client    dep.Client
	publisher pubsub.Publisher
}

type cursor struct {
	Value     string
	CreatedAt time.Time
}

// A cursor is valid for a week.
func (c cursor) Valid() bool {
	expiration := time.Now().Add(24 * 7 * time.Hour)
	if c.CreatedAt.After(expiration) {
		return false
	}
	return true
}

func New(client dep.Client, pub pubsub.Publisher) (Syncer, error) {
	sync := &watcher{
		publisher: pub,
		client:    client,
	}

	go func() {
		if err := sync.Run(); err != nil {
			log.Println("DEP watcher failed: ", err)
		}
	}()
	return sync, nil
}

// TODO this is private temporarily until the interface can be defined
func (w *watcher) privateDEPSyncer() bool {
	return true
}

func (w *watcher) Run() error {
	ticker := time.NewTicker(10 * time.Second).C
	cursor := ""
	for {
		resp, err := w.client.FetchDevices(dep.Limit(100), dep.Cursor(cursor))
		if err != nil {
			return err
		}
		fmt.Printf("more=%v, cursor=%s, fetched=%v\n", resp.MoreToFollow, resp.Cursor, resp.FetchedUntil)
		cursor = resp.Cursor
		e := NewEvent(resp.Devices)
		data, err := MarshalEvent(e)
		if err != nil {
			return err
		}
		if err := w.publisher.Publish(SyncTopic, data); err != nil {
			return err
		}
		if !resp.MoreToFollow {
			break
		}
		<-ticker
	}
	return nil
}
