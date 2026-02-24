# Worktree preparation target
# Called by devagent after creating a git worktree.
# Customize this for your project's specific needs.

.PHONY: worktree-prep
worktree-prep:
	@echo "Preparing Go worktree..."
	@# Download Go module dependencies
	go mod download
	@# Build to verify the worktree is functional
	go build ./...
	@echo "Worktree ready."
