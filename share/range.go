package share

// Range is an end exclusive set of share indexes.
type Range struct {
	// Start is the index of the first share occupied by this range.
	Start int
	// End is the next index after the last share occupied by this range.
	End int
}

func NewRange(start, end int) Range {
	return Range{Start: start, End: end}
}

func EmptyRange() Range {
	return Range{Start: 0, End: 0}
}

func (r Range) IsEmpty() bool {
	return r.Start == 0 && r.End == 0
}

func (r *Range) Add(value int) {
	r.Start += value
	r.End += value
}

// GetShareRangeForNamespace returns all shares that belong to a given
// namespace. It will return an empty range if the namespace could not be
// found. This assumes that the slice of shares are lexicographically
// sorted by namespace. Ranges here are always end exclusive.
func GetShareRangeForNamespace(shares []Share, ns Namespace) Range {
	if len(shares) == 0 {
		return EmptyRange()
	}
	n0 := shares[0].Namespace()
	if ns.IsLessThan(n0) {
		return EmptyRange()
	}
	n1 := shares[len(shares)-1].Namespace()
	if ns.IsGreaterThan(n1) {
		return EmptyRange()
	}

	start := -1
	for i, share := range shares {
		shareNS := share.Namespace()
		if shareNS.IsGreaterThan(ns) && start != -1 {
			return Range{start, i}
		}
		if ns.Equals(shareNS) && start == -1 {
			start = i
		}
	}
	if start == -1 {
		return EmptyRange()
	}
	return Range{start, len(shares)}
}
