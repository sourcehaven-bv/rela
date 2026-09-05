package sqlitestore

// Test-only accessors. They live here so the ladder's shape can be asserted
// without widening the package's real API for it.

// MigrationSteps reports the version each ladder rung produces, in order.
func MigrationSteps() []int {
	out := make([]int, len(migrations))
	for i, m := range migrations {
		out[i] = m.to
	}
	return out
}
