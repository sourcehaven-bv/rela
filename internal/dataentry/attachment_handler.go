package dataentry

import (
	"context"
	"net/http"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// attachmentHandler serves the entity-attachment routes
// (/api/v1/{plural}/{id}/_attachments/...): upload, download, detach.
// Extracted from App (TKT-R68TV8) to shrink the god object.
//
// It holds the full store.Store because attachment.New (the shared HTTP/CLI
// write-policy service) requires it. The write handle is NOT the full manager:
// this handler never calls it, only passes it to attachment.New, so it holds
// exactly attachment's own one-method attachment.EntityUpdater (TKT-IVSJV6). The
// swappable collaborators (acl, audit sink, field resolver, command runner) are closures over
// App so tests that reassign app.acl / app.fieldResolver after construction
// stay effective — same rationale as affordanceService. gateRead is App's
// shared uniform-404 read gate (handlers_attachment and the entity read path
// must 404 identically for hidden and nonexistent ids).
//
// writeMu is a POINTER to App's mutation mutex: attachment uploads/deletes
// mutate the owning entity's property, so they serialize against every other
// data-entry mutation handler. (Goes away when TKT-R68TV8 M5.4 moves write
// serialization behind the store.)
type attachmentHandler struct {
	schema     func() *Schema
	store      store.Store
	manager    attachment.EntityUpdater
	runner     func() attachment.CommandRunner
	reader     entityReader
	serializer entitySerializer
	acl        func() acl.ACL
	audit      func() audit.Audit
	fields     func() FieldVerdictResolver
	gateRead   func(w http.ResponseWriter, r *http.Request, typeName, entityID string) bool
	writeMu    *sync.Mutex

	// provision implements unmatched_principal: provision (TKT-ANUJDS), set by
	// App after construction. Called under writeMu at the top of each attachment
	// write; a no-op unless an unmatched verified principal hits a provision
	// policy. See writeHandler.enterWrite for the shared rationale.
	provision func(context.Context) context.Context
}

// enterWrite acquires writeMu and runs the provision seam under it, returning
// the request the handler must use. The caller defers Unlock itself.
func (h *attachmentHandler) enterWrite(r *http.Request) *http.Request {
	h.writeMu.Lock()
	if h.provision != nil {
		return r.WithContext(h.provision(r.Context()))
	}
	return r
}
