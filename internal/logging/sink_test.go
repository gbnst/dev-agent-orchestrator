// pattern: Imperative Shell

package logging

import (
	"encoding/json"
	"testing"
	"time"
)

func TestChannelSink_Write(t *testing.T) {
	sink := NewChannelSink(10)
	defer func() { _ = sink.Close() }()

	// Write a log entry as JSON (simulating what Zap sends)
	entry := map[string]any{
		"level":  "info",
		"ts":     time.Now().Unix(),
		"logger": "test.scope",
		"msg":    "test message",
		"fieldA": "valueA",
	}
	data, _ := json.Marshal(entry)
	data = append(data, '\n')

	n, err := sink.Write(data)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() = %d, want %d", n, len(data))
	}

	// Read from channel
	select {
	case got := <-sink.Entries():
		if got.Message != "test message" {
			t.Errorf("Message = %q, want %q", got.Message, "test message")
		}
		if got.Scope != "test.scope" {
			t.Errorf("Scope = %q, want %q", got.Scope, "test.scope")
		}
		if got.Level != "INFO" {
			t.Errorf("Level = %q, want %q", got.Level, "INFO")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for log entry")
	}
}

func TestChannelSink_NonBlocking(t *testing.T) {
	// Create sink with buffer size 2
	sink := NewChannelSink(2)
	defer func() { _ = sink.Close() }()

	entry := map[string]any{"level": "info", "msg": "test", "logger": "app"}
	data, _ := json.Marshal(entry)
	data = append(data, '\n')

	// Write 5 entries (more than buffer)
	for i := 0; i < 5; i++ {
		n, err := sink.Write(data)
		if err != nil {
			t.Fatalf("Write() error on iteration %d: %v", i, err)
		}
		if n != len(data) {
			t.Errorf("Write() = %d, want %d", n, len(data))
		}
	}

	// Should not block - oldest entries dropped
	// Drain what's available
	count := 0
	for {
		select {
		case <-sink.Entries():
			count++
		default:
			goto done
		}
	}
done:
	// Should have at most buffer size entries
	if count > 2 {
		t.Errorf("got %d entries, expected at most 2", count)
	}
}

func TestChannelSink_Sync(t *testing.T) {
	sink := NewChannelSink(10)
	defer func() { _ = sink.Close() }()

	// Sync should not error
	if err := sink.Sync(); err != nil {
		t.Errorf("Sync() error = %v", err)
	}
}

func TestChannelSink_Close(t *testing.T) {
	sink := NewChannelSink(10)
	_ = sink.Close()

	// Write after close should not panic
	_, err := sink.Write([]byte(`{"msg":"test"}`))
	if err == nil {
		t.Error("Write() after Close() should return error")
	}
}

func TestChannelSink_ConcurrentWriteClose(t *testing.T) {
	sink := NewChannelSink(10)

	entry := map[string]any{"level": "info", "msg": "concurrent", "logger": "test"}
	data, _ := json.Marshal(entry)
	data = append(data, '\n')

	// Writer goroutine: write in a tight loop until sink is closed
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			_, _ = sink.Write(data)
		}
	}()

	// Close after a brief moment to maximize overlap
	_ = sink.Close()

	// Wait for writer to finish — must not panic
	<-done
}

func TestChannelSink_Send(t *testing.T) {
	sink := NewChannelSink(10)
	defer func() { _ = sink.Close() }()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Scope:     "test.scope",
		Message:   "direct message",
		Fields:    make(map[string]any),
	}

	// Send should not block
	sink.Send(entry)

	// Read from channel
	select {
	case got := <-sink.Entries():
		if got.Message != "direct message" {
			t.Errorf("Message = %q, want %q", got.Message, "direct message")
		}
		if got.Scope != "test.scope" {
			t.Errorf("Scope = %q, want %q", got.Scope, "test.scope")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for log entry from Send()")
	}
}

func TestChannelSink_Send_NonBlocking(t *testing.T) {
	// Create sink with buffer size 2
	sink := NewChannelSink(2)
	defer func() { _ = sink.Close() }()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Scope:     "test",
		Message:   "test",
		Fields:    make(map[string]any),
	}

	// Send 5 entries (more than buffer)
	for i := 0; i < 5; i++ {
		sink.Send(entry)
	}

	// Should not block - oldest entries dropped
	count := 0
	for {
		select {
		case <-sink.Entries():
			count++
		default:
			goto done
		}
	}
done:
	// Should have at most buffer size entries
	if count > 2 {
		t.Errorf("got %d entries, expected at most 2", count)
	}
}

func TestChannelSink_Send_AfterClose(t *testing.T) {
	sink := NewChannelSink(10)
	_ = sink.Close()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Scope:     "test",
		Message:   "test",
		Fields:    make(map[string]any),
	}

	// Send after close should not panic
	sink.Send(entry)
}
