package pubsub

import "sync"

type Publisher interface {
	Publish(topic string, msg []byte) error
}

type Event struct {
	Topic   string
	Message []byte
}

func NewInmemPubsub() *Inmem {
	publish := make(chan Event)
	subscriptions := make(map[string][]subscription)
	inmem := &Inmem{
		publish:       publish,
		subscriptions: subscriptions,
	}
	go inmem.dispatch()
	return inmem
}

type Inmem struct {
	mtx           sync.RWMutex
	subscriptions map[string][]subscription

	publish chan Event
}

type subscription struct {
	name      string
	topic     string
	eventChan chan<- Event
}

func (p *Inmem) Publish(topic string, msg []byte) error {
	event := Event{Topic: topic, Message: msg}
	go func() { p.publish <- event }()
	return nil
}
