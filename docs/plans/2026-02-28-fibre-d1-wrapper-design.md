# Design: Replace PayForFibreHandler with FibreTx Internal Wrapper (D1)

## Problem

PR #223 introduced Fibre support using a `PayForFibreHandler` interface that
celestia-app must implement. This interface has two methods:

- `IsPayForFibreTx(tx []byte) bool` — detects PayForFibre transactions
- `CreateSystemBlob(tx []byte) (*share.Blob, error)` — creates the system blob

This approach couples go-square callers to a callback pattern that requires
celestia-app to decode SDK transactions on every call. More importantly, it
prevents go-square from self-contained detection of PayForFibre transactions,
unlike BlobTx which is detected by its `"BLOB"` type ID prefix.

The BlobTx wrapper caused querying issues in celestia-app because CometBFT
stores the wrapped bytes, and standard SDK decoders cannot parse them. The
D1 approach avoids this for Fibre: celestia-app wraps the MsgPayForFibre
transaction *only* during PrepareProposal/ProcessProposal (after CometBFT has
already stored the raw SDK tx bytes). CometBFT never sees the FibreTx wrapper,
so standard tx querying works.

## Design

### FibreTx Proto Message

Add to `proto/blob/v4/blob.proto`:

```protobuf
message FibreTx {
    bytes tx = 1;              // raw MsgPayForFibre SDK tx bytes
    BlobProto system_blob = 2; // commitment blob (share v2, 36 bytes data)
    string type_id = 3;        // "FIBR"
}
```

Reuses the existing `BlobProto` for the system blob. No Cosmos SDK or
Fibre-specific message types are needed. go-square treats the inner `tx` as
opaque bytes (same as BlobTx).

### tx Package

New file `tx/fibre_tx.go` mirroring `tx/blob_tx.go`:

- `ProtoFibreTxTypeID = "FIBR"`
- `type FibreTx struct { Tx []byte; SystemBlob *share.Blob }`
- `UnmarshalFibreTx(tx []byte) (*FibreTx, bool, error)` — three-return-value
  pattern matching `UnmarshalBlobTx`
- `MarshalFibreTx(tx []byte, systemBlob *share.Blob) ([]byte, error)` — used
  by celestia-app during PrepareProposal to wrap the SDK tx + system blob

### Delete PayForFibreHandler

Remove the `PayForFibreHandler` interface and all handler parameters from
public API functions:

- `Construct(txs, maxSquareSize, subtreeRootThreshold)` — no handler param
- `TxShareRange(txs, txIndex, maxSquareSize, subtreeRootThreshold)` — no handler
- `BlobShareRange(txs, txIndex, blobIndex, maxSquareSize, subtreeRootThreshold)` — no handler

### Detection Flow

`populateBuilder` and `validateTxOrdering` use the same type-discrimination
pattern as BlobTx:

```
for each tx:
    try UnmarshalBlobTx  → if isBlobTx, handle as PFB
    try UnmarshalFibreTx → if isFibreTx, handle as Fibre
    else                 → handle as normal tx
```

### Builder Changes

Merge `AppendPayForFibreTx` + `AppendSystemBlob` into a single atomic method:

```go
func (b *Builder) AppendFibreTx(fibreTx *tx.FibreTx) (bool, error)
```

This atomically adds the raw tx bytes to `PayForFibreTxs` (compact shares in
`PayForFibreNamespace`) and the system blob to `Blobs` (sparse shares). Remove
standalone `AppendSystemBlob`.

### What Stays the Same

- Square layout: `[TxNS | PfbNS | PayForFibreNS | padding | sparse blobs | tail]`
- Share v2 encoding, `NewV2Blob`, `FibreBlobVersion()`, `FibreCommitment()`
- `NoPfbIndex` sentinel for Fibre blobs in the `Element` struct
- All Builder internals for compact/sparse share counting and revert logic
- `PayForFibreNamespace` (0x05), `ShareVersionTwo`, `PayForFibreCounter`

### Data Flow

```
celestia-app PrepareProposal:
  1. Receives raw SDK txs from CometBFT (CometBFT stores these → queryable)
  2. Detects MsgPayForFibre using SDK message type detection
  3. Generates system blob (NewV2Blob with commitment)
  4. Wraps: MarshalFibreTx(sdkTxBytes, systemBlob) → FibreTx bytes
  5. Passes all txs (regular + BlobTx-wrapped + FibreTx-wrapped) to go-square

go-square Construct/Builder:
  1. Detects FibreTx by "FIBR" type ID (same pattern as BlobTx "BLOB")
  2. Places raw tx in PayForFibreNamespace compact shares
  3. Places system blob in user's namespace as sparse shares
  4. No IndexWrapper needed (system blob found by namespace)
```
