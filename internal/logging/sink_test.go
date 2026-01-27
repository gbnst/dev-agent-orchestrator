// pattern: Imperative Shell

package logging

import (
	"encoding/json"
	"testing"
	"time"
)

func TestChannelSink_Write(t *testing.T) {
	sink := NewChannelSink(10)
	defer sink.Close()

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
	defer sink.Close()

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
	defer sink.Close()

	// Sync should not error
	if err := sink.Sync(); err != nil {
		t.Errorf("Sync() error = %v", err)
	}
}

func TestChannelSink_Close(t *testing.T) {
	sink := NewChannelSink(10)
	sink.Close()

	// Write after close should not panic
	_, err := sink.Write([]byte(`{"msg":"test"}`))
	if err == nil {
		t.Error("Write() after Close() should return error")
	}
}
