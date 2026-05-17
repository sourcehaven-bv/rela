package entitymanager

import "github.com/Sourcehaven-BV/rela/internal/autocascade"

// NewCascadeHostForTest exposes the unexported cascadeHost so audit
// tests can pin its behavior without driving a full Runner.Process.
// Test-only; production code constructs cascadeHost per-call inside
// Manager methods.
func NewCascadeHostForTest(m *Manager) autocascade.Host {
	return &cascadeHost{deps: m.deps}
}
