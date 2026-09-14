package lock

// MemoryLockerEntries reports how many key entries the locker currently
// tracks. Exported to the external test package only, so the refcount
// bookkeeping (whose failure mode is an unbounded map, invisible from the
// public API) can be asserted directly.
func MemoryLockerEntries(l *MemoryLocker) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.held)
}
