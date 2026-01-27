//go:build e2e
// +build e2e

package e2e

import "testing"

func TestPodmanCreateContainer(t *testing.T) {
	testCreateContainer(t, "podman")
}

func TestPodmanStartStopContainer(t *testing.T) {
	testStartStopContainer(t, "podman")
}

func TestPodmanDestroyContainer(t *testing.T) {
	testDestroyContainer(t, "podman")
}

func TestPodmanCreateTmuxSession(t *testing.T) {
	testCreateTmuxSession(t, "podman")
}

func TestPodmanKillTmuxSession(t *testing.T) {
	testKillTmuxSession(t, "podman")
}

func TestPodmanTmuxAttachCommand(t *testing.T) {
	testTmuxAttachCommand(t, "podman")
}
