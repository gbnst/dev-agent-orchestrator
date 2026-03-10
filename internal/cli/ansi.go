// pattern: Functional Core
package cli

import "regexp"

// ansiPattern matches ANSI escape sequences (CSI sequences, OSC sequences, and simple escapes).
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b[()][0-9A-B]`)

// StripANSI removes ANSI escape sequences from the given string.
func StripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}
