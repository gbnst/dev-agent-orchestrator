// pattern: Functional Core
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestApp_PrintAgentHelp_NonEmpty(t *testing.T) {
	app := BuildApp("test", "")
	buf := &bytes.Buffer{}
	app.PrintAgentHelp(buf)

	if buf.Len() == 0 {
		t.Fatal("PrintAgentHelp produced no output")
	}
}

func TestApp_PrintAgentHelp_ContainsSectionHeaders(t *testing.T) {
	app := BuildApp("test", "")
	buf := &bytes.Buffer{}
	app.PrintAgentHelp(buf)
	output := buf.String()

	sections := []string{
		"AGENT ORCHESTRATION GUIDE",
		"OVERVIEW",
		"WORKFLOW",
		"COMMAND REFERENCE",
		"SESSION INTERACTION PATTERNS",
		"AGENTIC DEVELOPMENT PHASES",
		"MASTER AGENT PATTERN",
		"EXIT CODES",
	}

	for _, section := range sections {
		if !strings.Contains(output, section) {
			t.Errorf("output missing section header %q", section)
		}
	}
}

func TestApp_PrintAgentHelp_ContainsRealCommandNames(t *testing.T) {
	app := BuildApp("test", "")
	buf := &bytes.Buffer{}
	app.PrintAgentHelp(buf)
	output := buf.String()

	commands := []string{
		"list",
		"cleanup",
		"version",
		"worktree create",
		"container start",
		"container stop",
		"container destroy",
		"session create",
		"session destroy",
		"session readlines",
		"session send",
		"session tail",
	}

	for _, cmd := range commands {
		if !strings.Contains(output, cmd) {
			t.Errorf("output missing command %q", cmd)
		}
	}
}

func TestApp_PrintAgentHelp_ContainsRealUsageStrings(t *testing.T) {
	app := BuildApp("test", "")
	buf := &bytes.Buffer{}
	app.PrintAgentHelp(buf)
	output := buf.String()

	usageFragments := []string{
		"<container-id-or-name>",
		"<session-name>",
		"--no-start",
		"--no-color",
		"<id-or-name>",
		"<project-path>",
	}

	for _, fragment := range usageFragments {
		if !strings.Contains(output, fragment) {
			t.Errorf("output missing usage fragment %q", fragment)
		}
	}
}

func TestApp_PrintAgentHelp_ContainsExitCodes(t *testing.T) {
	app := BuildApp("test", "")
	buf := &bytes.Buffer{}
	app.PrintAgentHelp(buf)
	output := buf.String()

	for _, code := range []string{"0  Success", "1  Error", "2  No running"} {
		if !strings.Contains(output, code) {
			t.Errorf("output missing exit code %q", code)
		}
	}
}
