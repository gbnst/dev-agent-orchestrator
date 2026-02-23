// pattern: Imperative Shell

package worktree

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PatchComposeForWorktree modifies the worktree's docker-compose.yml to:
// 1. Add volume mounts for git overlay files
// 2. Set a unique top-level `name:` for compose project isolation
func PatchComposeForWorktree(projectPath, wtDir, name string) error {
	composePath := filepath.Join(wtDir, ".devcontainer", "docker-compose.yml")

	data, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("reading compose file: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing compose YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("invalid compose YAML structure")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("compose root is not a mapping")
	}

	// Find and patch the app service
	projectName := filepath.Base(projectPath)
	if err := patchAppServiceVolumes(root, projectPath); err != nil {
		return err
	}

	// Set top-level name for compose project isolation
	composeName := sanitizeComposeName(projectName + "-" + name)
	setTopLevelName(root, composeName)

	// Write back, preserving 2-space indent from templates
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("marshaling compose YAML: %w", err)
	}
	enc.Close()

	if err := os.WriteFile(composePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing compose file: %w", err)
	}

	return nil
}

// patchAppServiceVolumes finds the app service and appends git overlay volume mounts.
// The app service is identified by having devagent.project_path label but NOT devagent.sidecar_type.
func patchAppServiceVolumes(root *yaml.Node, projectPath string) error {
	servicesNode := findMapValue(root, "services")
	if servicesNode == nil {
		return fmt.Errorf("no 'services' key in compose file")
	}

	appServiceNode := findAppService(servicesNode)
	if appServiceNode == nil {
		return fmt.Errorf("no app service found (service with devagent.project_path but no devagent.sidecar_type)")
	}

	// Build volume mount strings
	mounts := buildVolumeMounts(projectPath)

	// Find or create volumes key
	volumesNode := findMapValue(appServiceNode, "volumes")
	if volumesNode == nil {
		// Add volumes key to service
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "volumes"}
		valNode := &yaml.Node{Kind: yaml.SequenceNode}
		appServiceNode.Content = append(appServiceNode.Content, keyNode, valNode)
		volumesNode = valNode
	}

	// Append our mounts
	for _, mount := range mounts {
		volumesNode.Content = append(volumesNode.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: mount,
		})
	}

	return nil
}

// buildVolumeMounts returns the volume mount strings for a worktree container.
// Strategy: mount the root .git directory at its host path inside the container.
// The worktree's .git file (already in the workspace mount) contains:
//
//	gitdir: <projectPath>/.git/worktrees/<name>
//
// This mount makes that host-path reference valid inside the container.
func buildVolumeMounts(projectPath string) []string {
	return []string{
		fmt.Sprintf("%s/.git:%s/.git:cached", projectPath, projectPath),
	}
}

// findAppService finds the app service node in the services mapping.
// App service has devagent.project_path label but NOT devagent.sidecar_type.
func findAppService(servicesNode *yaml.Node) *yaml.Node {
	if servicesNode.Kind != yaml.MappingNode {
		return nil
	}

	for i := 1; i < len(servicesNode.Content); i += 2 {
		serviceNode := servicesNode.Content[i]
		if serviceNode.Kind != yaml.MappingNode {
			continue
		}

		labelsNode := findMapValue(serviceNode, "labels")
		if labelsNode == nil {
			continue
		}

		hasProjectPath := false
		hasSidecarType := false

		if labelsNode.Kind == yaml.MappingNode {
			for j := 0; j < len(labelsNode.Content)-1; j += 2 {
				key := labelsNode.Content[j].Value
				if key == "devagent.project_path" {
					hasProjectPath = true
				}
				if key == "devagent.sidecar_type" {
					hasSidecarType = true
				}
			}
		}

		if hasProjectPath && !hasSidecarType {
			return serviceNode
		}
	}

	return nil
}

// findMapValue finds a value node for a given key in a mapping node.
func findMapValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// setTopLevelName sets or adds the top-level `name:` key in the compose file.
func setTopLevelName(root *yaml.Node, name string) {
	// Check if name already exists
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "name" {
			root.Content[i+1].Value = name
			return
		}
	}

	// Prepend name as first key-value pair
	nameKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "name"}
	nameVal := &yaml.Node{Kind: yaml.ScalarNode, Value: name}
	root.Content = append([]*yaml.Node{nameKey, nameVal}, root.Content...)
}

// sanitizeComposeName converts a name to a valid Docker Compose project name.
// Compose names must be lowercase, alphanumeric with hyphens.
func sanitizeComposeName(name string) string {
	name = strings.ToLower(name)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)
	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")
	return name
}
