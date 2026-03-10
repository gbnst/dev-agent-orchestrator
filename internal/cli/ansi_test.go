// pattern: Functional Core
package cli

import (
	"testing"
)

// TestStripANSI_RemovesColorCodes verifies that color codes are stripped.
func TestStripANSI_RemovesColorCodes(t *testing.T) {
	input := "\x1b[31mred text\x1b[0m"
	want := "red text"
	got := StripANSI(input)
	if got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, want)
	}
}

// TestStripANSI_RemovesBoldAndReset verifies that bold and reset codes are removed.
func TestStripANSI_RemovesBoldAndReset(t *testing.T) {
	input := "\x1b[1mBold\x1b[0m Normal"
	want := "Bold Normal"
	got := StripANSI(input)
	if got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, want)
	}
}

// TestStripANSI_PreservesPlainText verifies that plain text without escapes is unchanged.
func TestStripANSI_PreservesPlainText(t *testing.T) {
	input := "no escapes here"
	want := "no escapes here"
	got := StripANSI(input)
	if got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, want)
	}
}

// TestStripANSI_RemovesCursorMovement verifies that cursor movement codes are removed.
func TestStripANSI_RemovesCursorMovement(t *testing.T) {
	input := "\x1b[2J\x1b[H"
	want := ""
	got := StripANSI(input)
	if got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, want)
	}
}

// TestStripANSI_HandlesMultipleCodes verifies that mixed color and style codes are all removed.
func TestStripANSI_HandlesMultipleCodes(t *testing.T) {
	input := "\x1b[1m\x1b[32mGreen Bold\x1b[0m\x1b[4mUnderline\x1b[0m"
	want := "Green BoldUnderline"
	got := StripANSI(input)
	if got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, want)
	}
}
