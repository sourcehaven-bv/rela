package dataentry

import (
	"context"
	"net/http"

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
// exactly the manager's attachment surface (entitymanager.AttachmentsOf): the
// one write that may change a file value, and its lock (TKT-IVSJV6,
// BUG-CTUW2N). The swappable collaborators (acl, audit sink, field resolver,
// command runner) are closures over App so tests that reassign app.acl /
// app.fieldResolver after construction stay effective — same rationale as
// affordanceService. visible is the gated resolver every addressed route
// reads through, so hidden and nonexistent faces 404 identically.
type attachmentHandler struct {
	schema     func() *Schema
	store      store.Store
	runner     func() attachment.CommandRunner
	reader     entityReader
	visible    visibleReader
	serializer entitySerializer
	acl        func() acl.ACL
	audit      func() audit.Audit
	fields     func() FieldVerdictResolver
	// affordances applies the `fields:` write check, the same one PATCH and
	// PUT apply.
	affordances affordanceService
	// owner stamps file values and serializes writers to one (entity,
	// property). The manager's copy engine and face delete hold the same
	// lock, and so do the remote MCP attachment tools.
	owner attachmentOwner
	// uploads bounds concurrent uploads; shared with the remote MCP tools.
	uploads *attachment.Limiter

	// provision implements unmatched_principal: provision (TKT-ANUJDS), set by
	// App after construction. Called at the top of each attachment write; a
	// no-op unless an unmatched verified principal hits a provision policy.
	// See writeHandler.withProvision for the shared rationale.
	provision func(context.Context) context.Context
}

// attachmentOwner is the manager's attachment surface the handler needs;
// entitymanager.Attachments supplies it.
type attachmentOwner interface {
	attachment.Stamper
	attachment.Locker
}

// withProvision runs the provision seam and returns the request the handler
// must use.
func (h *attachmentHandler) withProvision(r *http.Request) *http.Request {
	if h.provision != nil {
		return r.WithContext(h.provision(r.Context()))
	}
	return r
}
