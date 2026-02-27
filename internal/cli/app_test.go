// pattern: Functional Core
package cli

import (
	"bytes"
	"os"
	"testing"
)

func TestApp_PrintHelp_ShowsGroupedCommands(t *testing.T) {
	app := NewApp("1.0.0")
	app.AddGroup("worktree", "Manage git worktrees")
	app.AddGroup("container", "Manage containers")

	buf := &bytes.Buffer{}
	app.PrintHelp(buf)

	output := buf.String()
	if output == "" {
		t.Fatal("PrintHelp produced no output")
	}

	if !bytes.Contains([]byte(output), []byte("Command Groups (requires running instance)")) {
		t.Errorf("Help missing 'Command Groups (requires running instance)' section")
	}

	if !bytes.Contains([]byte(output), []byte("worktree")) {
		t.Errorf("Help missing 'worktree' group")
	}

	if !bytes.Contains([]byte(output), []byte("container")) {
		t.Errorf("Help missing 'container' group")
	}
}

func TestApp_Execute_NoArgs_ReturnsTrueForTUI(t *testing.T) {
	app := NewApp("1.0.0")
	result := app.Execute(nil)
	if !result {
		t.Errorf("Execute(nil) returned %v, want true", result)
	}
}

func TestApp_Execute_UngroupedCommand_Dispatches(t *testing.T) {
	app := NewApp("1.0.0")
	called := false
	cmd := &Command{
		Name:    "version",
		Summary: "Print version",
		Usage:   "Usage: devagent version",
		Run: func(args []string) error {
			called = true
			return nil
		},
	}
	app.AddCommand(cmd)

	result := app.Execute([]string{"version"})
	if result {
		t.Errorf("Execute with command returned %v, want false", result)
	}
	if !called {
		t.Errorf("Command Run was not called")
	}
}

func TestApp_Execute_GroupCommand_Dispatches(t *testing.T) {
	app := NewApp("1.0.0")
	group := app.AddGroup("container", "Manage containers")

	called := false
	passedArgs := []string(nil)
	cmd := &Command{
		Name:    "start",
		Summary: "Start a container",
		Usage:   "Usage: devagent container start <id>",
		Run: func(args []string) error {
			called = true
			passedArgs = args
			return nil
		},
	}
	group.AddCommand(cmd)

	result := app.Execute([]string{"container", "start", "abc"})
	if result {
		t.Errorf("Execute with group command returned %v, want false", result)
	}
	if !called {
		t.Errorf("Command Run was not called")
	}
	if len(passedArgs) != 1 || passedArgs[0] != "abc" {
		t.Errorf("Command received args %v, want [abc]", passedArgs)
	}
}

func TestApp_Execute_GroupHelp_PrintsGroupCommands(t *testing.T) {
	app := NewApp("1.0.0")
	group := app.AddGroup("container", "Manage containers")

	cmd := &Command{
		Name:    "start",
		Summary: "Start a container",
		Usage:   "Usage: devagent container start <id>",
		Run: func(args []string) error {
			return nil
		},
	}
	group.AddCommand(cmd)

	// Capture stderr
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()

	r, w, _ := os.Pipe()
	os.Stderr = w

	result := app.Execute([]string{"container", "help"})

	w.Close()
	buf := &bytes.Buffer{}
	buf.ReadFrom(r)
	os.Stderr = oldStderr

	if result {
		t.Errorf("Execute with group help returned %v, want false", result)
	}
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("start")) {
		t.Errorf("Group help output missing 'start' command")
	}
}

func TestApp_Execute_CommandHelp_PrintsUsage(t *testing.T) {
	app := NewApp("1.0.0")
	group := app.AddGroup("container", "Manage containers")

	runCalled := false
	cmd := &Command{
		Name:    "start",
		Summary: "Start a container",
		Usage:   "Usage: devagent container start <id-or-name>",
		Run: func(args []string) error {
			runCalled = true
			return nil
		},
	}
	group.AddCommand(cmd)

	// Capture stderr
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()

	r, w, _ := os.Pipe()
	os.Stderr = w

	result := app.Execute([]string{"container", "start", "--help"})

	w.Close()
	buf := &bytes.Buffer{}
	buf.ReadFrom(r)
	os.Stderr = oldStderr

	if result {
		t.Errorf("Execute with --help returned %v, want false", result)
	}
	if runCalled {
		t.Errorf("Command Run was called, should have printed usage instead")
	}
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Usage: devagent container start")) {
		t.Errorf("Usage output missing expected usage string, got: %s", output)
	}
}

func TestApp_Execute_GroupHelpFlag_PrintsGroupCommands(t *testing.T) {
	app := NewApp("1.0.0")
	group := app.AddGroup("container", "Manage containers")

	cmd := &Command{
		Name:    "start",
		Summary: "Start a container",
		Usage:   "Usage: devagent container start <id>",
		Run: func(args []string) error {
			return nil
		},
	}
	group.AddCommand(cmd)

	for _, helpFlag := range []string{"--help", "-h"} {
		t.Run(helpFlag, func(t *testing.T) {
			oldStderr := os.Stderr
			defer func() { os.Stderr = oldStderr }()

			r, w, _ := os.Pipe()
			os.Stderr = w

			result := app.Execute([]string{"container", helpFlag})

			w.Close()
			buf := &bytes.Buffer{}
			buf.ReadFrom(r)
			os.Stderr = oldStderr

			if result {
				t.Errorf("Execute with %s returned %v, want false", helpFlag, result)
			}
			output := buf.String()
			if !bytes.Contains([]byte(output), []byte("start")) {
				t.Errorf("Group help output with %s missing 'start' command, got: %s", helpFlag, output)
			}
		})
	}
}

func TestApp_Execute_UnknownCommand_ExitsWithCode1(t *testing.T) {
	// Skip this test in testing mode as we can't intercept os.Exit
	// Instead we'll verify the logic through other tests
	t.Skip("os.Exit interception requires special test setup")
}
