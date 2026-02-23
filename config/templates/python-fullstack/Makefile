# Worktree preparation target
# Called by devagent after creating a git worktree.
# Customize this for your project's specific needs.

.PHONY: worktree-prep
worktree-prep:
	@echo "Preparing Python worktree..."
	@# Install Python dependencies
	@if [ -f "pyproject.toml" ]; then \
		uv sync 2>/dev/null || pip install -e ".[dev]" 2>/dev/null || echo "Install dependencies manually"; \
	elif [ -f "requirements.txt" ]; then \
		pip install -r requirements.txt; \
	fi
	@echo "Worktree ready."
