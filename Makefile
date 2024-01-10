DOCKER := $(shell which docker)
DOCKER_BUF := $(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace bufbuild/buf

## proto-gen-docker: Generate protobuf files. Requires docker.
proto-gen-docker:
	@echo "--> Generating Protobuf files"
	$(DOCKER_BUF) generate
.PHONY: proto-gen-docker

## proto-gen: Generate protobuf files. Requires buf.
proto-gen:
	@echo "--> Generating Protobuf files"
	@buf generate
.PHONY: proto-gen

## proto-lint-docker: Lint protobuf files. Requires docker.
proto-lint-docker:
	@echo "--> Linting Protobuf files"
	@$(DOCKER_BUF) lint
.PHONY: proto-lint
