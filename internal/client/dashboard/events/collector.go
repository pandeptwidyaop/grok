package events

import (
	"sync"

	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// EventCollector is the central hub for collecting and distributing events.
// It uses a buffered channel and observer pattern to avoid blocking tunnel operations.
type EventCollector struct {
	eventCh     chan Event
	subscribers []chan Event
	mu          sync.RWMutex
	stopCh      chan struct{}
	stopped     bool
}

// NewEventCollector creates a new event collector with a large buffer.
func NewEventCollector() *EventCollector {
	ec := &EventCollector{
		eventCh:     make(chan Event, 1000), // Large buffer to prevent blocking
		subscribers: make([]chan Event, 0),
		stopCh:      make(chan struct{}),
	}

	// Start background goroutine to process events
	go ec.run()

	return ec
}

// Publish publishes an event to all subscribers.
// This method is non-blocking and will drop events if the channel is full.
func (ec *EventCollector) Publish(event Event) {
	ec.mu.RLock()
	if ec.stopped {
		ec.mu.RUnlock()
		return
	}
	ec.mu.RUnlock()

	select {
	case ec.eventCh <- event:
		// Event sent successfully
	default:
		// Channel full, drop event and log warning
		logger.WarnEvent().
			Str("event_type", string(event.Type)).
			Msg("Event channel full, dropping event")
	}
}

// Subscribe creates a new subscription channel for receiving events.
// The caller is responsible for consuming from the channel to avoid blocking.
// Returns a buffered channel that will receive events.
func (ec *EventCollector) Subscribe() chan Event {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	// Create buffered channel for this subscriber
	ch := make(chan Event, 100)
	ec.subscribers = append(ec.subscribers, ch)

	return ch
}

// Unsubscribe removes a subscription channel.
func (ec *EventCollector) Unsubscribe(ch chan Event) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	for i, sub := range ec.subscribers {
		if sub == ch {
			// Remove from slice
			ec.subscribers = append(ec.subscribers[:i], ec.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// run processes events from the event channel and broadcasts to subscribers.
func (ec *EventCollector) run() {
	for {
		select {
		case event := <-ec.eventCh:
			ec.broadcast(event)
		case <-ec.stopCh:
			return
		}
	}
}

// broadcast sends an event to all subscribers without blocking.
func (ec *EventCollector) broadcast(event Event) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	for _, sub := range ec.subscribers {
		select {
		case sub <- event:
			// Event sent to subscriber
		default:
			// Subscriber channel full, skip
			logger.WarnEvent().
				Str("event_type", string(event.Type)).
				Msg("Subscriber channel full, skipping event")
		}
	}
}

// Close stops the event collector and closes all subscriber channels.
func (ec *EventCollector) Close() {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	if ec.stopped {
		return
	}

	ec.stopped = true
	close(ec.stopCh)

	// Close all subscriber channels
	for _, sub := range ec.subscribers {
		close(sub)
	}

	ec.subscribers = nil
}

// Size returns the current number of events in the queue.
func (ec *EventCollector) Size() int {
	return len(ec.eventCh)
}

// SubscriberCount returns the number of active subscribers.
func (ec *EventCollector) SubscriberCount() int {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return len(ec.subscribers)
}
