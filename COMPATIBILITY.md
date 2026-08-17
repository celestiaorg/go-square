# Compatibility

This document describes version compatibility between go-square, celestia-app, and share versions.

## Compatibility Matrix

| celestia-app | App Version | go-square | Share Versions |
|---|---|---|---|
| v2.x | 2 | v1.1.0 | v0 |
| v3.x | 3 | v1.1.1, v2.3.0 | v0, v1 |
| v4.x | 4 | v2.3.1 | v0, v1 |
| v5.x | 5 | v2.3.1 | v0, v1 |
| v6.x | 6 | v2.3.3, v3.0.2 | v0, v1 |
| v7.x | 7 | v2.3.3, v3.0.2 | v0, v1 |
| main | 8 | v2.3.3, v4.0.0-rc3 | v0, v1, v2 |

## Share Versions

- **v0**: Original format (blobs without signer).
- **v1**: Adds signer field (authored blobs). Introduced in go-square v2.
- **v2**: Adds Fibre blob version and commitment (Fibre system blobs). Introduced in go-square v4.

## Breaking Changes

### v4.0.0

Callers now classify transactions themselves. `Construct`, `NewBuilder`,
`TxShareRange`, and `BlobShareRange` take `[]ClassifiedTx` instead of
`[][]byte`, where a `ClassifiedTx` carries the raw transaction and, for
pay-for-fibre transactions, the system blob the caller synthesized.

`tx.TryParseFibreTx` and the `proto/cosmos/tx/v1beta1` and
`proto/celestia/fibre/v1` packages are removed. go-square decodes formats it
owns — `BlobTx`, `IndexWrapper`, `Blob` — and no longer decodes formats the
Cosmos SDK owns. Deciding whether a transaction is a pay-for-fibre transaction
requires the SDK's schema and the application's rules, so the application makes
that decision; a second classifier here could only agree with it by coincidence.

Classifications are validated: `Construct`, `NewBuilder`, `TxShareRange`, and
`BlobShareRange` return an error for a `ClassifiedTx` with empty bytes, a
fibre classification missing its system blob or whose bytes disagree, or a
`BlobTx` classified as a fibre transaction. Note that zero-length transactions
were previously accepted as normal transactions and are now rejected.

The deprecated `Build` is also removed. Use `NewBuilder` with `AppendTx`,
`AppendBlobTx`, `AppendFibreTx`, and `Export`.
