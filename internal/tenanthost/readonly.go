package tenanthost

import (
	"net/http"
	"net/url"
	"strings"
)

// ReadOnly serves a tenant whose state is read_only (TKT-59HWQ76): it
// passes reads to next and refuses every other request with 403
// `tenant_read_only`. A read is GET, HEAD, OPTIONS, or the WebDAV reads
// PROPFIND and REPORT.
//
// Two writes pass, because they change no tenant data:
//
//   - PUT on a live-update stream's subscriptions
//     (`/api/v1/_events/<id>/subscriptions`), which a browser needs to keep
//     its open views current;
//   - the IdP webhook ([WebhookPath]), which keeps the tenant's users in
//     step with the IdP. The IdP is not a tenant user, and a refusal would
//     make it retry without end.
//
// Everything else that writes is refused, including remote MCP, actions,
// comments, piles, leases, presence and settings. Background work the
// server starts itself, such as scheduled flows, is not a request and is
// not stopped, and neither is a read's own side effect: the user entity
// lazy provisioning creates on a person's first visit, and the edits a
// collaborative-editing session relays to other open clients, which it
// can no longer save.
func ReadOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if readOnlyAllows(r.Method, r.URL) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, r, http.StatusForbidden, "tenant_read_only", "This workspace is read-only")
	})
}

// readOnlyAllows reports whether a read-only tenant serves method on u.
// A write passes only on a path that is already clean and has no escapes:
// the router matches the escaped path, so a check on a decoded or cleaned
// one could pass a request the router sends elsewhere.
func readOnlyAllows(method string, u *url.URL) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, "PROPFIND", "REPORT":
		return true
	}
	if u.RawPath != "" || u.Path != cleanPath(u.Path) {
		return false
	}
	clean := u.Path
	switch method {
	case http.MethodPost:
		return clean == WebhookPath
	case http.MethodPut:
		rest, ok := strings.CutPrefix(clean, "/api/v1/_events/")
		if !ok {
			return false
		}
		id, tail, _ := strings.Cut(rest, "/")
		return id != "" && tail == "subscriptions"
	}
	return false
}
