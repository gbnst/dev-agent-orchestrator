// pattern: Imperative Shell

package discovery

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Scanner discovers projects in configured scan paths.
type Scanner struct{}

// NewScanner creates a new project scanner.
func NewScanner() *Scanner {
	return &Scanner{}
}

// ScanAll scans all provided paths for discoverable projects.
// Each path is walked one level deep looking for directories containing
// .devcontainer/docker-compose.yml with devagent.managed: "true" label.
func (s *Scanner) ScanAll(paths []string) []DiscoveredProject {
	var projects []DiscoveredProject
	seen := make(map[string]bool)

	for _, scanPath := range paths {
		entries, err := os.ReadDir(scanPath)
		if err != nil {
			continue // Skip inaccessible directories
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			projectPath := filepath.Join(scanPath, entry.Name())

			// Resolve symlinks to get canonical path
			resolved, err := filepath.EvalSymlinks(projectPath)
			if err != nil {
				resolved = projectPath
			}
			if seen[resolved] {
				continue
			}
			seen[resolved] = true

			if !isDevagentProject(resolved) {
				continue
			}

			project := DiscoveredProject{
				Name:        entry.Name(),
				Path:        resolved,
				HasMakefile: hasMakefile(resolved),
				Worktrees:   listWorktrees(resolved),
			}
			projects = append(projects, project)
		}
	}

	return projects
}

// isDevagentProject checks if a directory has .devcontainer/docker-compose.yml
// with devagent.managed: "true" label on any service.
func isDevagentProject(projectPath string) bool {
	composePath := filepath.Join(projectPath, ".devcontainer", "docker-compose.yml")
	return composeHasManagedLabel(composePath)
}

// composeHasManagedLabel parses a docker-compose.yml and checks if any service
// has the devagent.managed: "true" label.
func composeHasManagedLabel(composePath string) bool {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return false
	}

	// Parse the compose file structure just enough to check labels
	var compose struct {
		Services map[string]struct {
			Labels map[string]string `yaml:"labels"`
		} `yaml:"services"`
	}

	if err := yaml.Unmarshal(data, &compose); err != nil {
		return false
	}

	for _, svc := range compose.Services {
		if svc.Labels["devagent.managed"] == "true" {
			return true
		}
	}
	return false
}

// hasMakefile checks if a project has a Makefile at its root.
func hasMakefile(projectPath string) bool {
	_, err := os.Stat(filepath.Join(projectPath, "Makefile"))
	return err == nil
}

// listWorktrees runs `git worktree list --porcelain` and parses the output.
// Returns nil if not a git repo or no additional worktrees exist.
func listWorktrees(projectPath string) []Worktree {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = projectPath
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	return parseWorktreeList(string(output))
}

// parseWorktreeList parses the porcelain output of `git worktree list`.
// Format:
//
//	worktree /path/to/worktree
//	HEAD abc123
//	branch refs/heads/branch-name
//	<blank line>
//
// The first entry is the main worktree; we skip it and return only additional worktrees.
func parseWorktreeList(output string) []Worktree {
	var worktrees []Worktree
	var current *Worktree

	scanner := bufio.NewScanner(strings.NewReader(output))
	isFirst := true
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "worktree ") {
			// Save previous (non-first) worktree
			if current != nil && !isFirst {
				worktrees = append(worktrees, *current)
			}
			if current != nil {
				isFirst = false
			}
			path := strings.TrimPrefix(line, "worktree ")
			current = &Worktree{
				Path: path,
				Name: filepath.Base(path),
			}
		} else if strings.HasPrefix(line, "branch ") && current != nil {
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			current.Branch = branch
		} else if line == "" && current != nil {
			// End of entry
			if !isFirst {
				worktrees = append(worktrees, *current)
				current = nil
			} else {
				isFirst = false
				current = nil
			}
		}
	}

	// Handle last entry if no trailing newline
	if current != nil && !isFirst {
		worktrees = append(worktrees, *current)
	}

	return worktrees
}
