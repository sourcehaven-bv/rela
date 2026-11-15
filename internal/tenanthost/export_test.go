package tenanthost

import "time"

// SetProvisionClock replaces the clock the host's provisioning reads, so a
// test can pass the failure hold without waiting.
func SetProvisionClock(h *Host, now func() time.Time) {
	h.prov.mu.Lock()
	defer h.prov.mu.Unlock()
	h.prov.now = now
}

// ProvisionRunning reports whether a provisioning run for orgID is in
// progress on h.
func ProvisionRunning(h *Host, orgID string) bool {
	h.prov.mu.Lock()
	defer h.prov.mu.Unlock()
	_, ok := h.prov.runs[orgID]
	return ok
}
