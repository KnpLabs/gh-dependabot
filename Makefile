BINARY := gh-dependabot

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "} {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the binary
	go build -o $(BINARY) .

.PHONY: install
install: build ## Install the extension locally into gh
	gh extension install .

.PHONY: fmt
fmt: ## Format the code
	gofmt -w .

.PHONY: lint
lint: ## Run static checks (gofmt + go vet)
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then echo "Files not gofmt'd:"; echo "$$unformatted"; exit 1; fi
	go vet ./...

.PHONY: test
test: ## Run tests
	go test -race -cover ./...

.PHONY: tidy
tidy: ## Tidy go.mod / go.sum
	go mod tidy

.PHONY: check
check: lint test build ## Run lint, tests and build

.PHONY: clean
clean: ## Remove the built binary
	rm -f $(BINARY) $(BINARY).exe
