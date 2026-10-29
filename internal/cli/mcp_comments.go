package cli

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// stdioComments serves list_comments for `rela mcp`, and add_comment when
// the process runs as a `--run-as` job identity that may author
// ([comments.ValidAuthor]), such as a coding agent (TKT-RQKKS9). Without
// `--run-as` add_comment is not registered: which person a plain local
// session should comment as is a separate decision (TKT-QIQO1W), and an agent
// commenting as $USER would pass for the person. The edit, delete and accept
// tools are not registered on stdio.
//
// Reads and writes go through the stdio reader like every other stdio tool,
// with no ACL gate: the filesystem is the trust boundary
// ([newMCPServices]). The list flags that answer "may you" are false.
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

// newStdioCommentAdder returns add_comment's backing, or nil when the project
// declares no `comments:` block or runAs is not an identity that may author.
// An empty runAs is the plain local session.
func newStdioCommentAdder(
	svc *comments.Service, reader relamcp.GraphReader, meta *metamodel.Metamodel, runAs string,
) relamcp.CommentAdder {
	if svc == nil || runAs == "" || comments.ValidAuthor(runAs) != nil {
		return nil
	}
	return stdioComments{svc: svc, store: reader, meta: meta}
}

// AddComment adds a comment authored by the ctx principal. A bare id on an
// entity with content states must name the face, as on the HTTP route.
func (c stdioComments) AddComment(
	ctx context.Context, address string, in comments.AddInput,
) (comments.Comment, error) {
	ref, err := c.store.WriteTarget(ctx, address)
	var amb *visibility.AmbiguousAddressError
	switch {
	case errors.As(err, &amb):
		faces := make([]string, len(amb.Faces))
		for i, face := range amb.Faces {
			faces[i] = entity.FormatStateRef(amb.ID, face)
		}
		return comments.Comment{}, &comments.OpError{
			Code: "face_required", Title: "Address one face", Detail: amb.Error(), Faces: faces,
		}
	case err != nil:
		return comments.Comment{}, notFoundOr(err)
	}
	e, err := c.store.Resolve(ctx, entity.FormatStateRef(ref.ID, ref.Face))
	if err != nil || e == nil {
		return comments.Comment{}, notFoundOr(err)
	}
	if !metamodel.NewCommentPolicy(c.meta).Commentable(e.Type) {
		return comments.Comment{}, errCommentsNotEnabled
	}
	anchor, err := in.Anchor(e.Content)
	if err != nil {
		return comments.Comment{}, err
	}
	created, err := c.svc.Add(ctx, comments.Target{Type: e.Type, ID: e.ID, Face: e.Face},
		comments.AddRequest{Anchor: anchor, Body: in.Body})
	if refusal := comments.Refusal(err); refusal != nil {
		return comments.Comment{}, refusal
	}
	if err != nil {
		return comments.Comment{}, err
	}
	// Comments stay out of the audit log, so this line is the only record of
	// the OS user behind a `--run-as` author.
	slog.Info("mcp: comment added", "entity", entity.FormatStateRef(e.ID, e.Face), "comment", created.ID,
		"author", created.Author, "asserted_by", audit.AssertedByFrom(ctx))
	return created, nil
}

// errCommentsNotEnabled refuses a type the metamodel does not enable comments
// for.
var errCommentsNotEnabled = &comments.OpError{
	Code: "comments_not_enabled", Title: "Comments are not enabled for this entity type",
}

// notFoundOr returns the not-found refusal for a miss (or no entity), and err
// otherwise.
func notFoundOr(err error) error {
	if err == nil || errors.Is(err, store.ErrNotFound) {
		return &comments.OpError{Code: "not_found", Title: "Entity not found"}
	}
	return err
}

func (c stdioComments) ListComments(ctx context.Context, address string) ([]comments.View, error) {
	e, err := c.store.Resolve(ctx, address)
	if err != nil || e == nil {
		return nil, notFoundOr(err)
	}
	if !metamodel.NewCommentPolicy(c.meta).Commentable(e.Type) {
		return nil, errCommentsNotEnabled
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
