// pattern: Imperative Shell

package web

import (
	"fmt"
	"net/http"
	"sync"
)

// eventBroker fans out "state changed" signals to SSE subscribers.
type eventBroker struct {
	mu          sync.Mutex
	subscribers map[chan struct{}]struct{}
}

func newEventBroker() *eventBroker {
	return &eventBroker{
		subscribers: make(map[chan struct{}]struct{}),
	}
}

// Subscribe returns a buffered channel that receives a signal on each Notify call.
// The caller must call Unsubscribe when done.
func (b *eventBroker) Subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel and closes it.
func (b *eventBroker) Unsubscribe(ch chan struct{}) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
}

// Notify sends a signal to all subscribers. Non-blocking: if a subscriber's
// buffer is full (it hasn't consumed the previous signal), the new signal is
// coalesced — the subscriber will still re-fetch on the pending signal.
func (b *eventBroker) Notify() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// handleEvents is the SSE endpoint. It sends a "connected" event on open,
// then a "refresh" event each time the broker is notified.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.events.Subscribe()
	defer s.events.Unsubscribe(ch)

	// Send initial connected event.
	fmt.Fprintf(w, "event: connected\ndata: ok\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprintf(w, "event: refresh\ndata: update\n\n")
			flusher.Flush()
		}
	}
}
