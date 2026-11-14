package lua

import "testing"

// SwapMaxReadLimit lowers the read ceiling for one external test and
// restores it on cleanup. The caller must not be parallel; see
// swapMaxReadLimit.
func SwapMaxReadLimit(t *testing.T, n int) {
	t.Helper()
	t.Cleanup(swapMaxReadLimit(t, n))
}
