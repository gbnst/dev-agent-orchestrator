//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"devagent/internal/container"
)

func TestCreateWithCompose_Docker(t *testing.T) {
	testCreateWithCompose(t, "docker")
}

func TestCreateWithCompose_Podman(t *testing.T) {
	testCreateWithCompose(t, "podman")
}

func testCreateWithCompose(t *testing.T, runtime string) {
	SkipIfRuntimeMissing(t, runtime)
	SkipIfDevcontainerMissing(t)

	cfg := TestConfig(runtime)
	templates := TestTemplates()

	// Create a temporary project directory
	projectDir := TestProject(t, "basic")

	// Create manager
	mgr := container.NewManager(cfg, templates)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	opts := container.CreateOptions{
		ProjectPath: projectDir,
		Template:    "default",
		Name:        "test-compose-container",
	}

	// Create container using compose
	c, err := mgr.CreateWithCompose(ctx, opts)
	if err != nil {
		t.Fatalf("CreateWithCompose failed: %v", err)
	}

	// Cleanup - use standard Destroy which works for compose containers too
	// DestroyWithCompose is added in Phase 6; until then, standard Destroy works
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		// Standard Destroy will remove the container; compose down would be cleaner
		// but isn't available yet in Phase 5
		_ = mgr.Destroy(cleanupCtx, c.ID)
	})

	// Verify container was created
	if c == nil {
		t.Fatal("CreateWithCompose returned nil container")
	}

	displayID := c.ID
	if len(displayID) > 12 {
		displayID = displayID[:12]
	}
	t.Logf("Container created: %s", displayID)

	// Verify compose files were written
	composeFile := filepath.Join(projectDir, ".devcontainer", "docker-compose.yml")
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		t.Error("docker-compose.yml was not created")
	}

	dockerfileProxy := filepath.Join(projectDir, ".devcontainer", "Dockerfile.proxy")
	if _, err := os.Stat(dockerfileProxy); os.IsNotExist(err) {
		t.Error("Dockerfile.proxy was not created")
	}

	filterPy := filepath.Join(projectDir, ".devcontainer", "filter.py")
	if _, err := os.Stat(filterPy); os.IsNotExist(err) {
		t.Error("filter.py was not created")
	}

	devcontainerJson := filepath.Join(projectDir, ".devcontainer", "devcontainer.json")
	if _, err := os.Stat(devcontainerJson); os.IsNotExist(err) {
		t.Error("devcontainer.json was not created")
	}

	// Verify devcontainer.json has compose reference
	content, err := os.ReadFile(devcontainerJson)
	if err != nil {
		t.Fatalf("Failed to read devcontainer.json: %v", err)
	}

	if !strings.Contains(string(content), "dockerComposeFile") {
		t.Error("devcontainer.json missing dockerComposeFile property")
	}
}

func TestCreateWithCompose_BasicTemplate_Docker(t *testing.T) {
	testCreateWithComposeTemplate(t, "docker", "basic")
}

func TestCreateWithCompose_BasicTemplate_Podman(t *testing.T) {
	testCreateWithComposeTemplate(t, "podman", "basic")
}

func TestCreateWithCompose_GoProjectTemplate_Docker(t *testing.T) {
	testCreateWithComposeTemplate(t, "docker", "go-project")
}

func testCreateWithComposeTemplate(t *testing.T, runtime, templateName string) {
	SkipIfRuntimeMissing(t, runtime)
	SkipIfDevcontainerMissing(t)

	cfg := TestConfig(runtime)
	templates := TestTemplates()

	// Find the template
	var found bool
	for i := range templates {
		if templates[i].Name == templateName {
			found = true
			break
		}
	}
	if !found {
		t.Skipf("Template not found: %s", templateName)
	}

	projectDir := TestProject(t, templateName)

	mgr := container.NewManager(cfg, templates)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	opts := container.CreateOptions{
		ProjectPath: projectDir,
		Template:    templateName,
		Name:        "test-" + templateName + "-compose",
	}

	c, err := mgr.CreateWithCompose(ctx, opts)
	if err != nil {
		t.Fatalf("CreateWithCompose with %s template failed: %v", templateName, err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = mgr.DestroyWithCompose(cleanupCtx, c.ID)
	})

	if c == nil {
		t.Fatal("CreateWithCompose returned nil container")
	}

	displayID := c.ID
	if len(displayID) > 12 {
		displayID = displayID[:12]
	}
	t.Logf("Container created with %s template: %s", templateName, displayID)

	// Verify container is running
	if c.State != container.StateRunning {
		t.Errorf("Expected container state Running, got %s", c.State)
	}

	// Test proxy filtering works by verifying the compose file was created
	composeFile := filepath.Join(projectDir, ".devcontainer", "docker-compose.yml")
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		t.Error("docker-compose.yml was not created")
	}

	// Verify filter script exists (proxy sidecar configuration)
	filterPy := filepath.Join(projectDir, ".devcontainer", "filter.py")
	if _, err := os.Stat(filterPy); os.IsNotExist(err) {
		t.Error("filter.py was not created")
	}
}

func TestComposeLifecycle_Docker(t *testing.T) {
	testComposeLifecycle(t, "docker")
}

func TestComposeLifecycle_Podman(t *testing.T) {
	testComposeLifecycle(t, "podman")
}

func testComposeLifecycle(t *testing.T, runtime string) {
	SkipIfRuntimeMissing(t, runtime)
	SkipIfDevcontainerMissing(t)

	cfg := TestConfig(runtime)
	templates := TestTemplates()

	projectDir := TestProject(t, "basic")

	mgr := container.NewManager(cfg, templates)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	opts := container.CreateOptions{
		ProjectPath: projectDir,
		Template:    "default",
		Name:        "test-lifecycle-compose",
	}

	// Create container using compose
	c, err := mgr.CreateWithCompose(ctx, opts)
	if err != nil {
		t.Fatalf("CreateWithCompose failed: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = mgr.DestroyWithCompose(cleanupCtx, c.ID)
	})

	if c.State != container.StateRunning {
		t.Errorf("Expected container state Running after create, got %s", c.State)
	}

	displayID := c.ID
	if len(displayID) > 12 {
		displayID = displayID[:12]
	}
	t.Logf("Container created: %s", displayID)

	// Test StopWithCompose
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()

	err = mgr.StopWithCompose(stopCtx, c.ID)
	if err != nil {
		t.Fatalf("StopWithCompose failed: %v", err)
	}

	// Refresh to get updated state
	_ = mgr.Refresh(stopCtx)
	stoppedContainer, found := mgr.Get(c.ID)
	if !found {
		t.Fatal("Container not found after stop")
	}
	if stoppedContainer.State != container.StateStopped {
		t.Errorf("Expected container state Stopped after stop, got %s", stoppedContainer.State)
	}

	t.Log("Container stopped successfully")

	// Test StartWithCompose
	startCtx, startCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer startCancel()

	err = mgr.StartWithCompose(startCtx, c.ID)
	if err != nil {
		t.Fatalf("StartWithCompose failed: %v", err)
	}

	// Refresh to get updated state
	_ = mgr.Refresh(startCtx)
	startedContainer, found := mgr.Get(c.ID)
	if !found {
		t.Fatal("Container not found after start")
	}
	if startedContainer.State != container.StateRunning {
		t.Errorf("Expected container state Running after start, got %s", startedContainer.State)
	}

	t.Log("Container started successfully")

	// Test DestroyWithCompose (cleanup will handle this, but let's test it explicitly)
	destroyCtx, destroyCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer destroyCancel()

	err = mgr.DestroyWithCompose(destroyCtx, c.ID)
	if err != nil {
		t.Fatalf("DestroyWithCompose failed: %v", err)
	}

	// Verify container is gone
	_ = mgr.Refresh(destroyCtx)
	_, found = mgr.Get(c.ID)
	if found {
		t.Error("Container should not be found after destroy")
	}

	t.Log("Container destroyed successfully")
}
