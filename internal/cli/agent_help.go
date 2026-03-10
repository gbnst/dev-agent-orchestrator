// pattern: Functional Core
package cli

import (
	"fmt"
	"io"
	"maps"
	"slices"
)

// PrintAgentHelp prints a comprehensive guide for agent orchestration via the CLI.
// It combines static prose with dynamic command reference pulled from registered commands.
func (a *App) PrintAgentHelp(w io.Writer) {
	// Header
	fmt.Fprintln(w, "AGENT ORCHESTRATION GUIDE")
	fmt.Fprintln(w, "========================")
	fmt.Fprintln(w)

	// Overview
	fmt.Fprintln(w, "OVERVIEW")
	fmt.Fprintln(w, "--------")
	fmt.Fprintln(w, "Devagent manages containerized agentic development environments. It runs an")
	fmt.Fprintln(w, "interactive TUI that manages container lifecycles, tmux sessions, and git")
	fmt.Fprintln(w, "worktrees. A single devagent instance runs at a time (enforced by file lock).")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "All CLI commands delegate to the running TUI instance via HTTP. The TUI does")
	fmt.Fprintln(w, "not need to be in the foreground — it can run in a detached tmux session or")
	fmt.Fprintln(w, "terminal tab while a master agent drives work through the CLI.")
	fmt.Fprintln(w)

	// Workflow
	fmt.Fprintln(w, "WORKFLOW")
	fmt.Fprintln(w, "--------")
	fmt.Fprintln(w, "The typical agentic workflow uses worktree-based isolation:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  1. Create a worktree for the task:")
	fmt.Fprintln(w, "     devagent worktree create /path/to/project feature-name")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "     This creates a git worktree branch, spins up a container, and returns")
	fmt.Fprintln(w, "     JSON with the container ID and worktree path.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  2. Create a tmux session inside the container:")
	fmt.Fprintln(w, "     devagent session create <container-id> main")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  3. Send commands to the session:")
	fmt.Fprintln(w, "     devagent session send <container-id> main \"cd /workspace && make test\"")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  4. Read or tail the output:")
	fmt.Fprintln(w, "     devagent session readlines <container-id> main 500")
	fmt.Fprintln(w, "     devagent session tail <container-id> main --no-color")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  5. Repeat steps 3-4 as needed for the task.")
	fmt.Fprintln(w)

	// Dynamic command reference
	a.printCommandReference(w)

	// Session interaction patterns
	fmt.Fprintln(w, "SESSION INTERACTION PATTERNS")
	fmt.Fprintln(w, "---------------------------")
	fmt.Fprintln(w, "Send + Read pattern (polling):")
	fmt.Fprintln(w, "  devagent session send <container> <session> \"make build\"")
	fmt.Fprintln(w, "  sleep 5")
	fmt.Fprintln(w, "  devagent session readlines <container> <session> 500")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Send + Tail pattern (streaming):")
	fmt.Fprintln(w, "  devagent session send <container> <session> \"make test\"")
	fmt.Fprintln(w, "  devagent session tail <container> <session> --no-color")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Key behaviors:")
	fmt.Fprintln(w, "  - 'readlines N' captures the last N lines from scrollback history (default 20).")
	fmt.Fprintln(w, "  - 'send' auto-appends Enter (newline) to the text.")
	fmt.Fprintln(w, "  - 'tail' exits cleanly when the session ends or the container stops.")
	fmt.Fprintln(w, "  - Use '--no-color' with tail for machine-readable output (strips ANSI).")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Detecting command completion:")
	fmt.Fprintln(w, "  Poll with 'readlines' and look for your shell prompt pattern (e.g., '$ ' or")
	fmt.Fprintln(w, "  '# ') appearing after the command output. Alternatively, use 'tail' and")
	fmt.Fprintln(w, "  parse the streaming output for completion markers.")
	fmt.Fprintln(w)

	// Agentic development phases
	fmt.Fprintln(w, "AGENTIC DEVELOPMENT PHASES")
	fmt.Fprintln(w, "--------------------------")
	fmt.Fprintln(w, "  Design    — User defines requirements and high-level approach.")
	fmt.Fprintln(w, "  Plan      — User or agent creates a detailed implementation plan.")
	fmt.Fprintln(w, "  Implement — Master agent drives sessions: write code, run tests, iterate.")
	fmt.Fprintln(w, "  Review    — Agent or user reviews changes, runs final test suite.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "The user typically handles design and planning, then hands off to the master")
	fmt.Fprintln(w, "agent for implementation. The agent creates worktrees for isolation, drives")
	fmt.Fprintln(w, "coding sessions, and reports results back.")
	fmt.Fprintln(w)

	// Master agent pattern
	fmt.Fprintln(w, "MASTER AGENT PATTERN")
	fmt.Fprintln(w, "--------------------")
	fmt.Fprintln(w, "A master agent orchestrates one or more coding sessions concurrently:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  - Create separate worktrees for independent tasks.")
	fmt.Fprintln(w, "  - Run multiple sessions in parallel across containers.")
	fmt.Fprintln(w, "  - Monitor progress by polling 'session readlines' or using 'session tail'.")
	fmt.Fprintln(w, "  - Use 'devagent list' to get current state of all projects and containers.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "The user can take over any session at any time through the TUI or web UI,")
	fmt.Fprintln(w, "and hand it back to the agent by resuming CLI-driven interaction.")
	fmt.Fprintln(w)

	// Exit codes
	fmt.Fprintln(w, "EXIT CODES")
	fmt.Fprintln(w, "----------")
	fmt.Fprintln(w, "  0  Success")
	fmt.Fprintln(w, "  1  Error (invalid arguments, command failed, etc.)")
	fmt.Fprintln(w, "  2  No running devagent instance found")
}

// printCommandReference prints the dynamic command reference section
// by iterating registered commands and groups.
func (a *App) printCommandReference(w io.Writer) {
	fmt.Fprintln(w, "COMMAND REFERENCE")
	fmt.Fprintln(w, "-----------------")
	fmt.Fprintln(w)

	// Ungrouped commands in defined order
	fmt.Fprintln(w, "Top-level commands:")
	for _, name := range []string{"list", "cleanup", "version"} {
		if cmd, ok := a.commands[name]; ok {
			fmt.Fprintf(w, "  %-12s %s\n", cmd.Name, cmd.Summary)
			fmt.Fprintf(w, "               %s\n", cmd.Usage)
		}
	}
	fmt.Fprintln(w)

	// Groups in defined order
	for _, groupName := range []string{"worktree", "container", "session"} {
		group, ok := a.groups[groupName]
		if !ok {
			continue
		}
		fmt.Fprintf(w, "%s commands — %s:\n", group.Name, group.Summary)
		names := slices.Sorted(maps.Keys(group.Commands))
		for _, name := range names {
			cmd := group.Commands[name]
			fmt.Fprintf(w, "  %-12s %s\n", fmt.Sprintf("%s %s", groupName, cmd.Name), cmd.Summary)
			fmt.Fprintf(w, "               %s\n", cmd.Usage)
		}
		fmt.Fprintln(w)
	}
}
