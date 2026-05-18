package audit

// Nop is the no-op [Audit] backend. Tests that don't assert on audit
// records use it; production code paths that opt out (none today) can
// pass it too. Discards every Record without allocating.
type Nop struct{}

// Record discards rec.
func (Nop) Record(_ Record) {}
