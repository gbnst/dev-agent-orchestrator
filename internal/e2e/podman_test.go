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
