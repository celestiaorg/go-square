package share

// sequenceWriter fills shares without interpreting their payload. The caller
// initializes the first share's sequence metadata and handles any transaction
// delimiters, reserved offsets, and share ranges. Both public splitters use
// this writer so share boundaries and continuation headers have one implementation.
type sequenceWriter struct {
	shares  []Share
	pending *builder
}

// write appends payload and flushes full shares, leaving any partial share
// pending. In particular, an exact fit does not emit an empty trailing share.
func (w *sequenceWriter) write(data []byte) error {
	for {
		remaining := w.pending.AddData(data)
		if w.pending.AvailableBytes() == 0 {
			if err := w.flush(); err != nil {
				return err
			}
		}
		if remaining == nil {
			return nil
		}
		data = remaining
	}
}

// flush appends a full (or explicitly padded) share and prepares a continuation.
func (w *sequenceWriter) flush() error {
	share, err := w.pending.Build()
	if err != nil {
		return err
	}
	w.shares = append(w.shares, share)
	w.pending, err = newBuilder(w.pending.namespace, w.pending.shareVersion, false)
	return err
}

// finalize pads and flushes a partial share. An empty continuation is left
// pending so exact-fit payloads do not produce a trailing padding share.
func (w *sequenceWriter) finalize() (bytesOfPadding int, err error) {
	if w.pending.IsEmptyShare() {
		return 0, nil
	}
	bytesOfPadding = w.pending.ZeroPadIfNecessary()
	return bytesOfPadding, w.flush()
}
