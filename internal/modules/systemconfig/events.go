package systemconfig

import (
	"context"
	"sync"
)

type Subscriber func(context.Context, ConfigChanged)

type Bus struct {
	mu          sync.RWMutex
	subscribers []Subscriber
}

func NewBus() *Bus { return &Bus{} }

func (b *Bus) Subscribe(subscriber Subscriber) {
	if subscriber == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers = append(b.subscribers, subscriber)
}

func (b *Bus) Publish(ctx context.Context, event ConfigChanged) {
	b.mu.RLock()
	subscribers := append([]Subscriber(nil), b.subscribers...)
	b.mu.RUnlock()
	for _, subscriber := range subscribers {
		subscriber(ctx, event)
	}
}
