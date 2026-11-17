package tenant

// WaitClosing waits for every background close the registry has started, so
// a test can count closes after an eviction.
func (r *Registry[R]) WaitClosing() { r.closers.Wait() }
