# go-square

[![Go Reference](https://pkg.go.dev/badge/github.com/celestiaorg/go-square/v2.svg)](https://pkg.go.dev/github.com/celestiaorg/go-square/v2)

`go-square` is a Go module that provides data structures and utilities for interacting with data squares in the Celestia network. The data square is a special form of block serialization in the Celestia blockchain designed for sampling. This repo deals with the original data square which is distinct from the extended data square. Operations on the extended data square are handled by [rsmt2d](https://github.com/celestiaorg/rsmt2d).

Package   | Description
----------|---------------------------------------------------------------------------------------------------------------------
inclusion | Package inclusion contains functions to generate the blob share commitment from a given blob.
proto     | Package contains proto definitions and go generated code
share     | Package share contains encoding and decoding logic from blobs to shares.
square    | Package square implements the logic to construct the original data square based on a list of transactions.
tx        | Package tx contains BlobTx and IndexWrapper types

## Installation

To use `go-square` as a dependency in your Go project, you can use `go get`:

```bash
go get github.com/celestiaorg/go-square/v2
```

## Branches and Releasing

This repo has one long living development branch `main`, for changes targeting the next major version as well as long living branches for each prior major version i.e. `v1.x`. Non breaking changes may be backported to these branches. This repo follows [semver](https://www.semver.org) versioning.

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

This repo attempts to conform to [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/) so PR titles should ideally start with `fix:`, `feat:`, `build:`, `chore:`, `ci:`, `docs:`, `style:`, `refactor:`, `perf:`, or `test:` because this helps with semantic versioning and changelog generation. It is especially important to include an `!` (e.g. `feat!:`) if the PR includes a breaking change.

### Tools

1. Install [Go](https://golang.org/doc/install) 1.23.6
1. Install [golangci-lint](https://golangci-lint.run/usage/install/)
1. Fork this repo
1. Make your changes
1. Submit a pull request

### Helpful Commands

```sh
# Display all available make commands
make help

# Run tests
make test

# Run linter
make lint

# Perform benchmarking
make benchmark
```
