package cli

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// stdioComments serves list_comments for `rela mcp`. The write tools are
// not registered there (TKT-QIQO1W): stdio runs as the process's system
// principal, not as a signed-in user, and which person a local session
// should comment as is a separate decision.
//
// The read goes through the stdio reader like every other stdio tool. There
// is no principal to authorize, so the flags that answer "may you" are false.
type stdioComments struct {
	svc   *comments.Service
	store relamcp.GraphReader
	meta  *metamodel.Metamodel
}

// newStdioComments returns list_comments' backing, or nil when the project
// declares no `comments:` block.
func newStdioComments(
	svc *comments.Service, reader relamcp.GraphReader, meta *metamodel.Metamodel,
) relamcp.CommentReader {
	if svc == nil {
		return nil
	}
	return stdioComments{svc: svc, store: reader, meta: meta}
}

func (c stdioComments) ListComments(ctx context.Context, address string) ([]comments.View, error) {
	e, err := c.store.Resolve(ctx, address)
	if errors.Is(err, store.ErrNotFound) || (err == nil && e == nil) {
		return nil, &comments.OpError{Code: "not_found", Title: "Entity not found"}
	}
	if err != nil {
		return nil, err
	}
	if !metamodel.NewCommentPolicy(c.meta).Commentable(e.Type) {
		return nil, &comments.OpError{
			Code: "comments_not_enabled", Title: "Comments are not enabled for this entity type",
		}
	}
	list, err := c.svc.List(ctx, comments.Target{Type: e.Type, ID: e.ID, Face: e.Face})
	if err != nil {
		return nil, err
	}

	var declared map[string]metamodel.PropertyDef
	if def, ok := c.meta.GetEntityDef(e.Type); ok {
		declared = def.Properties
	}
	properties := comments.LiveProperties(declared, e.Properties)
	body := comments.NewBody(e.Content)
	out := make([]comments.View, len(list))
	for i, cm := range list {
		out[i] = comments.NewView(cm)
		out[i].Detached = comments.Detached(cm.Anchor, properties, body)
		out[i].Acceptable = !cm.Resolved && body.Acceptable(cm.Anchor)
	}
	return out, nil
}
