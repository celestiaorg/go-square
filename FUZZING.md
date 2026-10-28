# Fuzzing go-square

Run the seed corpus with the normal test suite:

```sh
go test -mod=readonly ./...
```

Run every fuzz target locally, or select one package and a longer budget:

```sh
make fuzz
make fuzz FUZZ_PACKAGE=./inclusion FUZZTIME=5m
go test -mod=readonly ./share -run '^$' -fuzz '^FuzzParseShares$' -fuzztime 5m
```

`scripts/fuzz.sh` discovers targets with `go test -list`, runs them sequentially,
and stops on the first failure. Each target defaults to 30 seconds and two
workers (`FUZZ_PARALLEL` overrides the worker count).

The [nightly workflow](.github/workflows/fuzz.yml) runs at 03:23 UTC and supports
manual dispatch. Separate square, share, inclusion, and transaction jobs run every
target in their package for five minutes each. The jobs restore the previous
mutation corpus from the Go fuzz cache and upload `testdata/fuzz` inputs on
failure. Normal PR CI executes all committed seeds via `go test ./...`.
Add new packages to the workflow matrix when they gain fuzz targets.

## Coverage inventory

This repository inventory supplies the go-square portion of
[PROTOCO-3321](https://linear.app/celestia/issue/PROTOCO-3321), whose cross-repository
inventory is still pending. It tracks the coverage added for
[PROTOCO-3328](https://linear.app/celestia/issue/PROTOCO-3328).
There are 13 targets across four packages. Coverage here means a target exercises
an entry point; it does not imply exhaustive branch coverage or a completed AI scan.

| Package / entry point | Input source | Fuzz targets | Properties | Last AI scan |
| --- | --- | --- | --- | --- |
| `share.NewShare`, `ParseShares`, `ParseTxs`, `ParseBlobs`, share accessors | Proposer-supplied raw shares | `FuzzParseShares` | Malformed namespaces, versions, lengths, offsets, delimiters, truncation, padding; no panic or input mutation | Not recorded |
| `share.CompactShareSplitter`, `ParseTxs`, share counter | Transaction payloads | `FuzzTxRoundTrip`, `FuzzSharesNeeded` | Round-trip, continuation suffixes, incremental export, capacity accounting and revert | Not recorded |
| `share.Blob.ToShares`, `ParseBlobs`, `ParseShares` | Blob payloads and signers | `FuzzBlobRoundTrip` | v0/v1 round-trip and share counts | Not recorded |
| `share.NewInfoByte`, `Sequence.validSequenceLen` | Share metadata | `FuzzNewInfoByte`, `FuzzValidSequenceLen` | Supported info bytes; accepted non-padding sequences have readable declared data | Not recorded |
| `square.Construct` | Ordered proposal transactions, including malformed BlobTx envelopes | `FuzzConstruct` | Reject invalid input without panicking; successful squares have bounded power-of-two dimensions, ordered namespaces, deterministic bytes | Not recorded |
| `square.Construct`, `BlobShareRange`, `Square.WrappedPFBs` | Valid transactions, v0/v1 PFB blobs, v2 Fibre system blobs | `FuzzConstructBlobRoundTrip` | Recover original transactions/blobs/signers; wrapper share indexes identify the PFB blob | Not recorded |
| `inclusion.GenerateSubtreeRoots`, `SubTreeWidth`; NMT `VerifyInclusion`, `VerifySubtreeRootInclusion` | v0/v1/v2 blob shares and untrusted proof fields | `FuzzCommitmentProofRoundTrip`, `FuzzVerifySubtreeRootInclusion` | Generated subtrees verify in a power-of-two NMT row; changed row digests fail; malformed proof ranges/nodes/roots do not panic | Not recorded |
| `tx.HasBlobTxTypeID`, `UnmarshalBlobTxNoCopy` | Untrusted protobuf envelopes | `FuzzHasBlobTxTypeID`, `FuzzUnmarshalBlobTxNoCopy`, `FuzzUnmarshalBlobTxNoCopyStructured` | Envelope verdict and canonical decode agree with reference protobuf decoder | Not recorded |

## Bounds and verification scope

The raw share target accepts at most 16 shares, square targets at most 32 KiB of
payloads and a 16-by-16 square, and the commitment target at most 16 KiB of blob
data. The raw proof target bounds each index/width to [-256, 256], the row root
to 256 bytes, and each encoded node list to 4 KiB. These limits bound fuzz work;
they are not protocol limits. Expected decode and capacity errors are accepted.
Panics are never recovered by the harness.

go-square generates commitments but does not implement a standalone proof
verifier. The inclusion targets exercise the pinned NMT dependency using the
outer namespace prefix and power-of-two tree required by Celestia's subtree
proof API. They do not cover celestia-app's or celestia-node's proof assembly,
verification wrappers, or data availability header checks. The structured square
target covers one PFB blob and an optional Fibre transaction; additional targets
can extend this to many blobs and interleaved builder append/revert operations.

## Crash triage and regression inputs

1. Download the failing job's `fuzz-failures-*` artifact and copy its files to the
   same package's `testdata/fuzz/<target>/` directory.
2. Reproduce with `go test ./path/to/package -run 'FuzzTarget/inputHash'`.
   Go's failure output includes this exact command and minimizes the input when
   the fuzz budget allows.
3. File a bug linked to PROTOCO-3328 with the target, commit SHA, Go version,
   workflow URL, reproducer, and stack trace. Distinguish a harness assertion
   error from a production crash before filing. Use the project's private
   security reporting process for sensitive findings.
4. Commit the minimized input with each fix (and a focused regression test when
   needed). `go test ./...` must pass with the input before closing the bug.
