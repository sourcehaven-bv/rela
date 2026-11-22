package dataentry

import "net/http"

// TenantStatusPath is the path a multi-tenant host answers with the
// caller's tenant status (tenanthost.StatusPath). The host answers it before
// the tenant's router sees the request, so the handler here only serves a
// single-tenant server.
const TenantStatusPath = "/api/v1/_tenant"

// singleTenantStatus is the status body of a server that serves one
// workspace: always active, never read-only.
type singleTenantStatus struct {
	Status string `json:"status"`
}

// handleV1TenantStatus answers GET /api/v1/_tenant on a single-tenant
// server. The SPA reads the path at start to learn whether the workspace is
// read-only. Without this route a single-tenant server answered 404 on every
// page load, which the browser logs as an error (BUG-ZL4K7QW).
func handleV1TenantStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	writeV1JSON(w, http.StatusOK, singleTenantStatus{Status: "active"})
}
