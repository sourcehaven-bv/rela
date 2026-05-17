package audit

import "sync"

// Memory is an in-memory [Audit] backend used by integration tests
// that need to assert on the audit stream without hitting the
// filesystem.
//
// Memory does NOT sanitize control chars or truncate fields — it
// retains the raw [Record] verbatim so tests can verify what the
// Manager passed in. The [Filesystem] backend applies sanitization;
// see plan AC15.
type Memory struct {
	mu      sync.Mutex
	records []Record
}

// NewMemory constructs an empty Memory backend.
func NewMemory() *Memory {
	return &Memory{}
}

// Record appends rec under the backend's internal mutex.
func (m *Memory) Record(rec Record) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, rec)
}

// Records returns a snapshot of the records recorded so far. The
// returned slice is independent — mutating it does not affect the
// backend's state, and subsequent Record calls do not mutate the
// returned slice.
func (m *Memory) Records() []Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Record, len(m.records))
	copy(out, m.records)
	return out
}
