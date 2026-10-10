package search

import "testing"

// SetFieldCheckPageForTest sets how many hits one field check judges
// together, for the duration of t.
func SetFieldCheckPageForTest(t *testing.T, n int) {
	t.Helper()
	prev := fieldCheckPage.Load()
	fieldCheckPage.Store(int64(n))
	t.Cleanup(func() { fieldCheckPage.Store(prev) })
}
