DOCKER := $(shell which docker)
DOCKER_BUF := $(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace bufbuild/buf:1.28.1

## proto-gen-docker: Generate protobuf files. Requires docker.
proto-gen:
	@echo "--> Generating Protobuf files"
	$(DOCKER_BUF) generate
.PHONY: proto-gen-docker

## proto-lint-docker: Lint protobuf files. Requires docker.
proto-lint:
	@echo "--> Linting Protobuf files"
	@$(DOCKER_BUF) lint
.PHONY: proto-lint
