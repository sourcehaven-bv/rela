package dataentry

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// MCPComments runs the comment operations for the remote MCP tools
// (TKT-QIQO1W). Each method is the HTTP route's operation with the same gates,
// in the same order; only the transport differs. A refusal is a
// [*comments.OpError] carrying the HTTP route's code and title.
//
// The ctx must be the remote MCP request's: it carries the principal, the ACL
// request, the read gate and the world that the `/api/` middleware stamps,
// and the gates read all four from it.
type MCPComments struct {
	h *commentsHandler
}

// address parses an MCP address into the route's (type, ref) pair. The type
// is the stored row's, read only when some face of the entity is readable,
// so an unreadable entity and an absent one answer alike. An address the
// route would refuse as a path is refused before any storage read.
func (c MCPComments) address(ctx context.Context, address string) (commentAddress, *commentFailure) {
	parsed, err := entity.ParseAddress(address)
	if err != nil || !isSafeStateRefSegment(address) {
		return commentAddress{}, failEntityNotFound()
	}
	typeName, err := c.h.visibleReader.readableType(ctx, parsed.ID())
	if err != nil {
		return commentAddress{}, failGate(err)
	}
	if typeName == "" {
		return commentAddress{}, failEntityNotFound()
	}
	if fail := c.h.commentEnabled(typeName); fail != nil {
		return commentAddress{}, fail
	}
	addr, ok := parseCommentAddress(typeName, address)
	if !ok {
		return commentAddress{}, failEntityNotFound()
	}
	return addr, nil
}

// The write methods refuse a read-only instance before resolving the address,
// as the routes do before reading the target.

// ListComments is the list route.
func (c MCPComments) ListComments(ctx context.Context, address string) ([]comments.View, error) {
	addr, fail := c.address(ctx, address)
	if fail != nil {
		return nil, fail.opError(ctx)
	}
	list, fail := c.h.listOp(ctx, addr)
	if fail != nil {
		return nil, fail.opError(ctx)
	}
	out := make([]comments.View, len(list))
	for i, l := range list {
		out[i] = l.view
	}
	return out, nil
}

// AddComment is the create route.
func (c MCPComments) AddComment(ctx context.Context, address string, in comments.AddInput) (comments.Comment, error) {
	if refused := c.h.readOnlyFailure(); refused != nil {
		return comments.Comment{}, refused.opError(ctx)
	}
	addr, fail := c.address(ctx, address)
	if fail != nil {
		return comments.Comment{}, fail.opError(ctx)
	}
	target, ent, fail := c.h.addGate(ctx, addr)
	if fail != nil {
		return comments.Comment{}, fail.opError(ctx)
	}
	created, fail := c.h.addOp(ctx, target, ent, in)
	if fail != nil {
		return comments.Comment{}, fail.opError(ctx)
	}
	return created, nil
}

// UpdateComment is the PATCH route.
func (c MCPComments) UpdateComment(
	ctx context.Context, address, commentID string, body *string, resolved *bool,
) error {
	if refused := c.h.readOnlyFailure(); refused != nil {
		return refused.opError(ctx)
	}
	addr, fail := c.address(ctx, address)
	if fail != nil {
		return fail.opError(ctx)
	}
	target, existing, fail := c.h.mutationGate(ctx, addr, commentID, mutationUpdate)
	if fail != nil {
		return fail.opError(ctx)
	}
	if fail := c.h.updateOp(ctx, target, existing, commentID, body, resolved); fail != nil {
		return fail.opError(ctx)
	}
	return nil
}

// DeleteComment is the DELETE route.
func (c MCPComments) DeleteComment(ctx context.Context, address, commentID string) error {
	if refused := c.h.readOnlyFailure(); refused != nil {
		return refused.opError(ctx)
	}
	addr, fail := c.address(ctx, address)
	if fail != nil {
		return fail.opError(ctx)
	}
	if fail := c.h.deleteOp(ctx, addr, commentID); fail != nil {
		return fail.opError(ctx)
	}
	return nil
}

// AcceptSuggestion is the accept route, provisioned as the route is: before
// the address is resolved, so a first-time user whose reads come through
// their provisioned user entity can see the target.
func (c MCPComments) AcceptSuggestion(ctx context.Context, address, commentID string) (comments.Accepted, error) {
	if refused := c.h.readOnlyFailure(); refused != nil {
		return comments.Accepted{}, refused.opError(ctx)
	}
	ctx = c.h.provisioned(ctx)
	addr, fail := c.address(ctx, address)
	if fail != nil {
		return comments.Accepted{}, fail.opError(ctx)
	}
	accepted, fail := c.h.acceptOp(ctx, addr, commentID)
	if fail != nil {
		return comments.Accepted{}, fail.opError(ctx)
	}
	return accepted, nil
}

// mcpComments returns the comment operations for the MCP host, or nil when
// commenting is off.
func mcpComments(h *commentsHandler) *MCPComments {
	if h == nil || h.svc == nil {
		return nil
	}
	return &MCPComments{h: h}
}

// provisioned runs the provision seam on ctx; see [commentsHandler.withProvision].
func (h *commentsHandler) provisioned(ctx context.Context) context.Context {
	if h.provision != nil {
		return h.provision(ctx)
	}
	return ctx
}
