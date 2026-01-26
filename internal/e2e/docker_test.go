//go:build e2e
// +build e2e

package e2e

import "testing"

func TestDockerCreateContainer(t *testing.T) {
	testCreateContainer(t, "docker")
}

func TestDockerStartStopContainer(t *testing.T) {
	testStartStopContainer(t, "docker")
}

func TestDockerDestroyContainer(t *testing.T) {
	testDestroyContainer(t, "docker")
}
