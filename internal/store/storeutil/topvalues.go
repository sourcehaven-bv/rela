package storeutil

import "sort"

// TopValues ranks distinct property values by frequency for
// [store.EntityReader.PropertyValues]: most common first, ties broken
// alphabetically so the result is deterministic across backends and across
// runs (Go map iteration order is not).
//
// limit <= 0 means "all values". That convention is load-bearing and easy to
// get subtly wrong: the pre-allocation must size to the result, not to limit,
// or the unlimited case allocates zero and grows by repeated reallocation.
// fsstore and memstore had exactly that bug while pgstore did not — three
// copies of the same twenty lines, one already drifted. Hoisting it here is
// what stops a fourth backend inheriting whichever copy it happened to read.
//
// Nil: a nil or empty counts map yields a non-nil empty slice, matching what
// the store contract's callers expect from a property with no values.
func TopValues(counts map[string]int, limit int) []string {
	type valueCount struct {
		value string
		count int
	}
	sorted := make([]valueCount, 0, len(counts))
	for v, c := range counts {
		sorted = append(sorted, valueCount{value: v, count: c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		return sorted[i].value < sorted[j].value
	})

	n := len(sorted)
	if limit > 0 && limit < n {
		n = limit
	}
	result := make([]string, 0, n)
	for i := range n {
		result = append(result, sorted[i].value)
	}
	return result
}
