package web

import (
	"testing"
	"time"
)

func TestEventBroker_SubscribeNotify(t *testing.T) {
	b := newEventBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	b.Notify()

	select {
	case <-ch:
		// ok
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected signal on subscriber channel")
	}
}

func TestEventBroker_MultipleSubscribers(t *testing.T) {
	b := newEventBroker()
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()
	defer b.Unsubscribe(ch1)
	defer b.Unsubscribe(ch2)

	b.Notify()

	for i, ch := range []chan struct{}{ch1, ch2} {
		select {
		case <-ch:
			// ok
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("subscriber %d: expected signal", i)
		}
	}
}

func TestEventBroker_CoalescesSignals(t *testing.T) {
	b := newEventBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	// Notify twice without consuming — second should be coalesced.
	b.Notify()
	b.Notify()

	// Drain the single buffered signal.
	select {
	case <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected at least one signal")
	}

	// Channel should be empty now.
	select {
	case <-ch:
		t.Fatal("expected channel to be empty after draining")
	default:
		// ok
	}
}

func TestEventBroker_UnsubscribeRemoves(t *testing.T) {
	b := newEventBroker()
	ch := b.Subscribe()
	b.Unsubscribe(ch)

	b.Notify()

	// Channel should not receive since we unsubscribed.
	select {
	case <-ch:
		t.Fatal("should not receive after unsubscribe")
	default:
		// ok
	}
}
