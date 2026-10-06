package audit

// Nop is the no-op [Audit] backend. Tests that don't assert on audit
// records use it, and so does the desktop app, which has one user and no
// operator to read a log. Discards every Record without allocating.
type Nop struct{}

// Record discards rec.
func (Nop) Record(_ Record) {}
