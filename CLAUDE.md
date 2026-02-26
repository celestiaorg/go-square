# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

go-square is a Go library for data square construction and encoding in the Celestia network. It handles the original data square (not the extended data square, which is handled by rsmt2d). Module path: `github.com/celestiaorg/go-square/v3`.

## Common Commands

```sh
make test           # Run unit tests: go test -mod=readonly ./...
make lint           # Run golangci-lint (requires golangci-lint installed)
make fmt            # Auto-fix lint and markdown formatting issues
make benchmark      # Run benchmarks: go test -mod=readonly -bench=. ./...
make proto-gen      # Regenerate protobuf Go code (requires docker)
make proto-lint     # Lint protobuf definitions (requires docker)
```

Run a single test:
```sh
go test -run TestName ./path/to/package
```

CI runs tests with race detection: `go test ./... -v -timeout 5m -race`

## Architecture

### Package Layout

- **square** (root): Constructs the original data square from transactions. `Builder` accumulates transactions, then `Build()` or `Construct()` assembles them into a `Square` (a `[]share.Share` arranged as a power-of-2 grid).
- **share**: Core encoding/decoding. Defines `Share` (512-byte unit), `Blob`, `Namespace` (29 bytes: 1 version + 28 ID). Handles splitting blobs into compact shares (transactions) and sparse shares (blobs), and parsing them back.
- **inclusion**: Generates blob share commitments using Merkle mountain ranges and namespaced Merkle trees (NMT). Uses `MerkleRootFn` callback for flexibility.
- **tx**: Marshaling/unmarshaling of `BlobTx` (transaction + attached blobs) and `IndexWrapper` (transaction + share indices). Uses protobuf with magic type ID prefixes (`"BLOB"`, `"INDX"`).
- **proto/blob/v2**: Protobuf definitions and generated code for `BlobProto`, `BlobTx`, `IndexWrapper`.
- **internal/test**: Test factories (`GenerateTxs`, `GenerateBlobTxs`, `GenerateBlobs`) used across packages.

### Key Data Flow

Transactions/blobs → `Builder` splits into compact/sparse shares → shares assembled into square (power-of-2 dimensions) → `inclusion` generates commitments from share subtree roots.

## Conventions

- **Commits**: Follow [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/) — prefix with `fix:`, `feat:`, `chore:`, `perf:`, `refactor:`, etc. Use `!` for breaking changes (e.g., `feat!:`).
- **Testing**: Uses `testify` (assert/require). Table-driven tests with `testCases` structs. Test packages use `_test` suffix.
- **Formatting**: gofumpt (via golangci-lint), not just gofmt.
- **Linting**: golangci-lint v2 with copyloopvar, gocritic, misspell, nakedret (max-func-lines: 0), prealloc, revive, staticcheck.
- **Protobuf**: Generated via `buf` (dockerized). Do not edit `*.pb.go` files manually.
