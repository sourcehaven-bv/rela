package tenant

// WaitClosing waits for every background close the registry has started, so
// a test can count closes after an eviction.
func (r *Registry[R]) WaitClosing() { r.closers.Wait() }

// wait blocks until orgID's current create ends, for tests.
func (a *AutoProvisioner) wait(orgID string) {
	a.mu.Lock()
	j, ok := a.jobs[orgID]
	a.mu.Unlock()
	if ok {
		<-j.done
	}
}
