DOCKER := $(shell which docker)
DOCKER_BUF := $(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace bufbuild/buf:1.28.1
PROJECTNAME=$(shell basename "$(PWD)")

.PHONY: help
help: Makefile ## Display all available make commands.
	@echo " Choose a command to run in "$(PROJECTNAME)":"
	@sed -n -e '/^.PHONY: /{N; s/^.PHONY: \(.*\)\n.*:.*## \(.*\)/\1:\2/; p}' $(MAKEFILE_LIST) | \
	sort | \
	awk 'BEGIN {FS = ":"; } {printf " \033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: proto-gen-docker
proto-gen: ## Generate protobuf files. Requires docker.
	@echo "--> Generating Protobuf files"
	$(DOCKER_BUF) generate

.PHONY: proto-lint
proto-lint: ## Lint protobuf files. Requires docker.
	@echo "--> Linting Protobuf files"
	@$(DOCKER_BUF) lint

.PHONY: lint
lint: ## Lint source code. Requires golangci-lint.
	@echo "--> Lint source code using golangci-lint"
	@golangci-lint run

.PHONY: test
test: ## Run unit tests.
	@echo "--> Run unit tests"
	@go test -mod=readonly ./...

.PHONY: benchmark
benchmark: ## Perform benchmark.
	@echo "--> Perform benchmark"
	@go test -mod=readonly -bench=. ./...