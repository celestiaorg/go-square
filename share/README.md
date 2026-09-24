# Shares

See the celestia-app specs for [shares](https://celestiaorg.github.io/celestia-app/specs/shares.html).

Both `CompactShareSplitter` and `SparseShareSplitter` use an internal sequence
writer to fill shares and construct continuation headers. The compact splitter
adds transaction length delimiters, records the first transaction offset in each
share's reserved bytes, and tracks transaction share ranges. The sparse splitter
starts a sequence for each blob, including its signer when required. Fibre blob
data is already encoded and passes through the same writer.

The exported splitter APIs and wire format are unchanged. Parsing still has
separate entry points: `ParseTxs` supports partial share ranges and skips leading
transaction continuations, while `ParseBlobs` expects complete blob sequences.
