package appbuild

// HasVersionSweep reports whether s assembled a version-reconciliation sweep.
// Test-only: it lets the external tests check the wiring through New without
// widening Services.
func HasVersionSweep(s *Services) bool { return s.versionSweep != nil }
