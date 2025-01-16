package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"

	"github.com/micromdm/micromdm/dep"
	conf "github.com/micromdm/micromdm/platform/config"
	"github.com/micromdm/micromdm/platform/pubsub"
)

const (
	SyncTopic = "mdm.DepSync"

	syncDuration        = 30 * time.Minute
	cursorValidDuration = 7 * 24 * time.Hour
)

type Syncer interface{ SyncNow() }

type WatcherDB interface {
	LoadCursor() (*Cursor, error)
	SaveCursor(c Cursor) error
	LoadAutoAssigners() ([]AutoAssigner, error)
}

type Watcher struct {
	mtx    sync.RWMutex
	logger log.Logger
	client Client

	publisher pubsub.Publisher
	db        WatcherDB
	startSync chan bool
	syncNow   chan bool

	cursor Cursor
}

func NewWatcher(db WatcherDB, pub pubsub.PublishSubscriber, opts ...Option) (*Watcher, error) {
	w := Watcher{
		logger:    log.NewNopLogger(),
		db:        db,
		publisher: pub,
		startSync: make(chan bool),
		syncNow:   make(chan bool),
	}
	for _, optFn := range opts {
		optFn(&w)
	}

	cursor, err := w.db.LoadCursor()
	if err != nil {
		return nil, err
	}
	if cursor.Valid() {
		level.Debug(w.logger).Log("msg", "loaded DEP config", "cursor", cursor.Value)
		w.cursor = *cursor
	}

	if err := w.updateClient(pub); err != nil {
		return nil, err
	}

	saveCursor := func() {
		if err := db.SaveCursor(w.cursor); err != nil {
			level.Info(w.logger).Log("err", err, "msg", "saving cursor")
			return
		}
		level.Info(w.logger).Log("msg", "saved DEP config", "cursor", w.cursor.Value)
	}

	go func() {
		defer saveCursor()
		if w.client == nil {
			// block until we have a DEP client to start sync process
			level.Info(w.logger).Log("msg", "waiting for DEP token to be added before starting sync")
			<-w.startSync
		}
		err := w.Run()
		// the DEP sync should never end without an error, but log
		// unconditionally anyway so we never silently stop watching
		level.Info(w.logger).Log("err", err, "msg", "DEP watcher stopped")
	}()

	return &w, nil
}

type Client interface {
	FetchDevices(...dep.DeviceRequestOption) (*dep.DeviceResponse, error)
	SyncDevices(string, ...dep.DeviceRequestOption) (*dep.DeviceResponse, error)
	AssignProfile(string, ...string) (*dep.ProfileResponse, error)
}

type Option func(*Watcher)

func WithClient(client Client) Option {
	return func(w *Watcher) {
		w.client = client
	}
}

func WithLogger(logger log.Logger) Option {
	return func(w *Watcher) {
		w.logger = logger
	}
}

func (w *Watcher) updateClient(pubsub pubsub.Subscriber) error {
	tokenAdded, err := pubsub.Subscribe(context.TODO(), "token-events", conf.DEPTokenTopic)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event := <-tokenAdded:
				var token conf.DEPToken
				if err := json.Unmarshal(event.Message, &token); err != nil {
					level.Info(w.logger).Log("err", err, "msg", "unmarshalling tokenAdd to token")
					continue
				}

				client, err := token.Client()
				if err != nil {
					level.Info(w.logger).Log("err", err, "msg", "creating new DEP client")
					continue
				}

				w.mtx.Lock()
				w.client = client
				w.mtx.Unlock()
				go func() { w.startSync <- true }() // unblock Run
			}
		}
	}()
	return nil
}

func (w *Watcher) SyncNow() {
	if w.client == nil {
		level.Info(w.logger).Log("msg", "waiting for DEP token to be added before starting sync")
		return
	}
	w.syncNow <- true
}

// TODO this needs to be a proper error in the micromdm/dep package.
func isCursorExhausted(err error) bool {
	return strings.Contains(err.Error(), "EXHAUSTED_CURSOR")
}

func isCursorExpired(err error) bool {
	return strings.Contains(err.Error(), "EXPIRED_CURSOR")
}

func isCursorInvalid(err error) bool {
	return strings.Contains(err.Error(), "INVALID_CURSOR")
}

// Process DEP messages and pull out filter-matching serial numbers
// associated to profile UUIDs for auto-assignment.
func (w *Watcher) filteredAutoAssignments(devices []dep.Device) (map[string][]string, error) {
	// load auto-assigners every run to make sure we get the latest set of
	// auto-assigner profile UUIDs/filters. Note this makes every *watcher
	// (i.e. every DEP sync instance) share the current DB set of auto-
	// assigners. perhaps to refactor to be more separated.
	assigners, err := w.db.LoadAutoAssigners()
	if err != nil {
		return nil, err
	}
	assigned := make(map[string][]string)
	// skip looping over serials if we have no autoassigners
	if len(assigners) < 1 {
		return assigned, nil
	}
	for _, d := range devices {
		// only process DEP "added" OpType messages
		if d.OpType != "added" {
			continue
		}
		// filter our devices by our assigner filters and get list of
		// which devices are to be assigned to which profiles
		for _, assigner := range assigners {
			if assigner.Filter == "*" { // only supported filter type right now
				if serials, ok := assigned[assigner.ProfileUUID]; ok {
					assigned[assigner.ProfileUUID] = append(serials, d.SerialNumber)
				} else {
					assigned[assigner.ProfileUUID] = []string{d.SerialNumber}
				}
			}
		}

	}
	return assigned, nil
}

func (w *Watcher) processAutoAssign(devices []dep.Device) error {
	assignments, err := w.filteredAutoAssignments(devices)
	if err != nil {
		return err
	}

	for profileUUID, serials := range assignments {
		resp, err := w.client.AssignProfile(profileUUID, serials...)
		if err != nil {
			level.Info(w.logger).Log(
				"err", err,
				"msg", "auto-assign error assigning serials to profile",
				"profile", profileUUID,
			)
			continue
		}
		// count our results for logging
		resultCounts := map[string]int{
			"SUCCESS":        0,
			"NOT_ACCESSIBLE": 0,
			"FAILED":         0,
		}
		for _, result := range resp.Devices {
			if ct, ok := resultCounts[result]; ok {
				// NOTE: we're logging _only_ the above pre-defined result types
				resultCounts[result] = ct + 1
			}
		}
		// TODO: alternate strategy is to log all failed devices
		// TODO: handle/requeue failed devices?
		level.Info(w.logger).Log(
			"msg", "DEP auto-assigned",
			"profile", profileUUID,
			"success", resultCounts["SUCCESS"],
			"not_accessible", resultCounts["NOT_ACCESSIBLE"],
			"failed", resultCounts["FAILED"],
		)
	}

	return nil
}

func (w *Watcher) publishAndProcessDevices(devices []dep.Device) error {
	e := NewEvent(devices)
	data, err := MarshalEvent(e)
	if err != nil {
		return err
	}
	err = w.publisher.Publish(context.TODO(), SyncTopic, data)
	if err != nil {
		return err
	}

	// TODO: instead of directly kicking off the auto-assigner process
	// consider placing a subscriber on the DEP pubsub topic. The same
	// information gets marshalled but it allows us the future
	// flexibility to separate out that component if we desired.
	go func() {
		err := w.processAutoAssign(devices)
		if err != nil {
			level.Info(w.logger).Log("err", err, "msg", "auto-assign error")
		}
	}()
	return nil
}

func (w *Watcher) Run() error {
	var (
		err       error
		resp      *dep.DeviceResponse
		fetchNext = true
		// for logging
		fetchNextLabel = map[bool]string{
			true:  "fetch",
			false: "sync",
		}
	)

	ticker := time.NewTicker(syncDuration).C
	for {
		if fetchNext {
			resp, err = w.client.FetchDevices(dep.Limit(100), dep.Cursor(w.cursor.Value))
			if err != nil && isCursorExhausted(err) {
				level.Info(w.logger).Log(
					"msg", "DEP cursor returned all devices previously",
					"phase", fetchNextLabel[fetchNext],
					"cursor", w.cursor.Value,
				)
				fetchNext = false
				continue
			}
		} else {
			resp, err = w.client.SyncDevices(w.cursor.Value)
		}

		if err != nil && (isCursorExpired(err) || isCursorInvalid(err)) {
			level.Info(w.logger).Log(
				"msg", "DEP cursor error, retrying with empty cursor",
				"phase", fetchNextLabel[fetchNext],
				"cursor", w.cursor.Value,
				"err", err,
			)
			w.cursor.Value = ""
			fetchNext = true
			continue
		} else if err != nil {
			// log any other error, but do not return from the run loop.
			// probably just a transient network issue.
			level.Info(w.logger).Log(
				"msg", "error syncing DEP devices",
				"phase", fetchNextLabel[fetchNext],
				"cursor", w.cursor.Value,
				"err", err,
			)
		} else {
			level.Info(w.logger).Log(
				"msg", "DEP sync",
				"phase", fetchNextLabel[fetchNext],
				"cursor", resp.Cursor,
				"fetched", resp.FetchedUntil,
				"devices", len(resp.Devices),
				"more", resp.MoreToFollow,
			)

			if err := w.publishAndProcessDevices(resp.Devices); err != nil {
				return fmt.Errorf("publish and process devices: %w", err)
			}
			w.cursor = Cursor{Value: resp.Cursor, CreatedAt: time.Now()}
			if err := w.db.SaveCursor(w.cursor); err != nil {
				return errors.Wrap(err, "saving cursor from fetch")
			}

			if resp.MoreToFollow {
				if resp.Cursor != w.cursor.Value {
					continue
				}
				level.Info(w.logger).Log(
					"msg", "DEP returned the same cursor that was sent, exiting sync early",
					"phase", fetchNextLabel[fetchNext],
					"cursor", w.cursor.Value,
					"fetched", resp.FetchedUntil,
					"more", resp.MoreToFollow,
				)
			} else if fetchNext {
				fetchNext = false
				continue
			}
		}

		select {
		case <-ticker:
		case <-w.syncNow:
			level.Info(w.logger).Log("msg", "explicit DEP sync requested")
		}
	}
}
