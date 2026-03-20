.DEFAULT_GOAL := help

# ══════════════════════════════════════════════════════════════════
# Variables
# ══════════════════════════════════════════════════════════════════

BINARY := jiru
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"
GOLANGCI_LINT := golangci-lint
WORKTREE_DIR := ../jiru-worktrees

# Colours for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
CYAN := \033[0;36m
NC := \033[0m

# Test flags
ifdef VERBOSE
	TEST_FLAGS := -v
endif

# ══════════════════════════════════════════════════════════════════
# Build
# ══════════════════════════════════════════════════════════════════

.PHONY: build build-all clean install

build: ## Build for current platform
	go build $(LDFLAGS) -o $(BINARY) .

build-all: clean ## Cross-compile for all platforms
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 .

clean: ## Clean build artefacts
	rm -f $(BINARY)
	rm -rf dist/

install: ## Install to $$GOPATH/bin
	go install $(LDFLAGS) .

# ══════════════════════════════════════════════════════════════════
# Quality
# ══════════════════════════════════════════════════════════════════

.PHONY: test coverage lint lint-fix fmt tidy vet check

test: ## Run tests (VERBOSE=1 for detailed output)
	go test -race $(TEST_FLAGS) ./...

coverage: ## Generate test coverage report (open in browser)
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@printf "\n$(CYAN)Opening HTML report...$(NC)\n"
	go tool cover -html=coverage.out
	@rm -f coverage.out

lint: ## Run linter
	$(GOLANGCI_LINT) run

lint-fix: ## Run linter with auto-fix
	$(GOLANGCI_LINT) run --fix

fmt: ## Format Go code
	go fmt ./...

tidy: ## Tidy Go modules
	go mod tidy

vet: ## Run go vet
	go vet ./...

check: fmt tidy vet lint test ## Run all checks (fmt, tidy, vet, lint, test)

# ══════════════════════════════════════════════════════════════════
# Worktrees
# ══════════════════════════════════════════════════════════════════

.PHONY: worktree-create worktree-list worktree-remove worktree-prune

# Extract branch name from positional arg (e.g., make worktree-create my-feature)
WORKTREE_TARGETS := worktree-create worktree-remove
BRANCH_ARG := $(word 2,$(MAKECMDGOALS))
ifneq ($(filter $(firstword $(MAKECMDGOALS)),$(WORKTREE_TARGETS)),)
  ifdef BRANCH_ARG
    BRANCH ?= $(BRANCH_ARG)
  endif
endif

worktree-create: ## Create a worktree (make worktree-create <branch> [BASE=main])
	@if [ -z "$(BRANCH)" ]; then \
		printf "$(RED)Error:$(NC) branch name is required\n"; \
		printf "  Usage: make worktree-create my-feature [BASE=main]\n"; \
		exit 1; \
	fi
	@set -e; \
	BASE=$${BASE:-main}; \
	WTDIR=$$(echo "$(BRANCH)" | tr '/' '-'); \
	WTPATH="$(WORKTREE_DIR)/$$WTDIR"; \
	mkdir -p "$(WORKTREE_DIR)"; \
	if [ -d "$$WTPATH" ]; then \
		printf "$(RED)Error:$(NC) Worktree already exists at $(YELLOW)$$WTPATH$(NC)\n"; \
		exit 1; \
	fi; \
	printf "$(CYAN)Creating worktree$(NC) $(YELLOW)$(BRANCH)$(NC) from $(YELLOW)$$BASE$(NC)...\n"; \
	if git show-ref --verify --quiet "refs/heads/$(BRANCH)" 2>/dev/null; then \
		git worktree add "$$WTPATH" "$(BRANCH)"; \
	elif git show-ref --verify --quiet "refs/remotes/origin/$(BRANCH)" 2>/dev/null; then \
		git worktree add "$$WTPATH" "$(BRANCH)"; \
	else \
		git worktree add -b "$(BRANCH)" "$$WTPATH" "$$BASE"; \
	fi; \
	printf "$(CYAN)Running$(NC) go mod download...\n"; \
	cd "$$WTPATH" && go mod download; \
	printf "\n$(GREEN)Worktree ready!$(NC)\n"; \
	printf "  $(YELLOW)cd$(NC) $$WTPATH\n"; \
	printf "  $(YELLOW)make$(NC) build\n\n"

worktree-list: ## List all worktrees
	@printf "$(CYAN)Worktrees:$(NC)\n\n"
	@git worktree list | while IFS= read -r line; do \
		if echo "$$line" | grep -q '\[.*\]'; then \
			path=$$(echo "$$line" | awk '{print $$1}'); \
			branch=$$(echo "$$line" | grep -o '\[.*\]' | tr -d '[]'); \
			if [ "$$path" = "$$(git rev-parse --show-toplevel 2>/dev/null)" ]; then \
				printf "  $(GREEN)%-50s$(NC) %s $(GREEN)(main)$(NC)\n" "$$path" "$$branch"; \
			else \
				printf "  $(YELLOW)%-50s$(NC) %s\n" "$$path" "$$branch"; \
			fi; \
		else \
			printf "  %s\n" "$$line"; \
		fi; \
	done
	@printf "\n"

worktree-remove: ## Remove a worktree (make worktree-remove <branch>)
	@if [ -z "$(BRANCH)" ]; then \
		printf "$(RED)Error:$(NC) branch name is required\n"; \
		printf "  Usage: make worktree-remove my-feature\n"; \
		exit 1; \
	fi
	@set -e; \
	WTDIR=$$(echo "$(BRANCH)" | tr '/' '-'); \
	WTPATH="$(WORKTREE_DIR)/$$WTDIR"; \
	if [ ! -d "$$WTPATH" ]; then \
		printf "$(RED)Error:$(NC) No worktree at $(YELLOW)$$WTPATH$(NC)\n"; \
		exit 1; \
	fi; \
	printf "$(CYAN)Removing worktree$(NC) $(YELLOW)$(BRANCH)$(NC)...\n"; \
	git worktree remove "$$WTPATH"; \
	printf "$(GREEN)Worktree removed.$(NC)\n"; \
	printf "  To delete the branch: $(YELLOW)git branch -d $(BRANCH)$(NC)\n\n"

# Swallow positional args so Make doesn't treat them as targets
%:
	@:

worktree-prune: ## Clean up stale worktree references
	@printf "$(CYAN)Pruning stale worktrees...$(NC)\n"
	@git worktree prune -v
	@printf "$(GREEN)Done.$(NC)\n\n"

# ══════════════════════════════════════════════════════════════════
# Help
# ══════════════════════════════════════════════════════════════════

help: ## Show this help message
	@printf "\n$(CYAN)jiru$(NC) — Makefile targets\n\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'
	@printf "\n"
