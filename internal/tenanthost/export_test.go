package tenanthost

import "time"

// SetProvisionClock replaces the clock the host's provisioning reads, so a
// test can pass the failure hold without waiting.
func SetProvisionClock(h *Host, now func() time.Time) {
	h.prov.mu.Lock()
	defer h.prov.mu.Unlock()
	h.prov.now = now
}
