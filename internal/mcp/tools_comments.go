package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/comments"
)

// CommentReader reads comment threads for list_comments (TKT-QIQO1W). The
// address is `ID` or `ID@face`; the implementation resolves the entity type
// itself, so a caller cannot name a type that disagrees with the stored row.
//
// The remote wiring passes the data-entry API's own operations, so a tool
// runs the same read floor, `comment:*` permissions, face checks and
// read-only refusal as the HTTP route. A refusal is a [*comments.OpError].
type CommentReader interface {
	ListComments(ctx context.Context, address string) ([]comments.View, error)
}

// CommentWriter backs the four comment write tools. Addresses and refusals
// are as for [CommentReader].
type CommentWriter interface {
	AddComment(ctx context.Context, address string, in comments.AddInput) (comments.Comment, error)
	// UpdateComment edits a comment. A nil field keeps its stored value.
	UpdateComment(ctx context.Context, address, commentID string, body *string, resolved *bool) error
	DeleteComment(ctx context.Context, address, commentID string) error
	AcceptSuggestion(ctx context.Context, address, commentID string) (comments.Accepted, error)
}

// commentHandler serves the comment tools.
type commentHandler struct {
	reader CommentReader
	writer CommentWriter
}

func selComments(h handlerSet) commentHandler { return h.comments }

// registerCommentTools registers the comment tools: list_comments when
// [Deps.Comments] is set, the write tools when [Deps.CommentWriter] is. A
// project without a `comments:` block lists none; stdio `rela mcp`, which
// runs as the system principal rather than a signed-in user, lists only
// list_comments.
func (s *Server) registerCommentTools() {
	d := s.deps()
	if d.Comments != nil {
		addTool(s, toolListComments(), commentTool(s, false, commentHandler.handleListComments))
	}
	if d.CommentWriter == nil {
		return
	}
	addTool(s, toolAddComment(), commentTool(s, true, commentHandler.handleAddComment))
	addTool(s, toolUpdateComment(), commentTool(s, true, commentHandler.handleUpdateComment))
	addTool(s, toolDeleteComment(), commentTool(s, true, commentHandler.handleDeleteComment))
	addTool(s, toolAcceptCommentSuggestion(), commentTool(s, true, commentHandler.handleAcceptSuggestion))
}

// commentTool binds a comment handler. A reload can drop the comment deps
// after the tools were registered; the tools then answer that commenting is
// off instead of calling a nil interface.
func commentTool(
	s *Server, writes bool,
	method func(commentHandler, context.Context, *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error),
) mcpgo.ToolHandler {
	return bind(s, selComments, func(
		h commentHandler, ctx context.Context, req *mcpgo.CallToolRequest,
	) (*mcpgo.CallToolResult, error) {
		if h.reader == nil || (writes && h.writer == nil) {
			return errorResult("comments are not enabled in this project"), nil
		}
		return method(h, ctx, req)
	})
}

const commentAddressDescription = "Entity ID, or ID@face for an entity with content states. " +
	"Writes to such an entity must name the face."

func toolListComments() *mcpgo.Tool {
	return newTool("list_comments",
		withDescription("List the comments on an entity, oldest first. Each has its anchor, body, "+
			"resolved state, and whether the anchor still resolves (detached), you may edit or delete it, "+
			"and its suggested replacement can be accepted."),
		withString("id", required(), description(commentAddressDescription)),
	)
}

func toolAddComment() *mcpgo.Tool {
	return newTool("add_comment",
		withDescription("Add a comment to an entity. Anchor it to a property, a view section, or a passage "+
			"of the body (text). A text comment may suggest replacement text for its quote."),
		withString("id", required(), description(commentAddressDescription)),
		withString("body", required(), description("Comment text (markdown)")),
		withString("anchor", required(), enum("property", "section", "text"),
			description("What the comment is about")),
		withString("ref", description("Property name (anchor=property) or section slug (anchor=section)")),
		withString("quote", description("anchor=text: the passage as displayed, without markdown markup")),
		withString("quote_prefix", description("anchor=text: text just before the quote, to pick one occurrence")),
		withString("quote_suffix", description("anchor=text: text just after the quote, to pick one occurrence")),
		withString("replacement",
			description("anchor=text: suggested replacement for the quote; empty suggests deleting it")),
	)
}

func toolUpdateComment() *mcpgo.Tool {
	return newTool("update_comment",
		withDescription("Edit a comment's body, or resolve or reopen it. Omitted fields keep their value; "+
			"pass at least one of body and resolved."),
		withString("id", required(), description(commentAddressDescription)),
		withString("comment_id", required(), description("Comment ID, as returned by list_comments")),
		withString("body", description("New comment text")),
		withBoolean("resolved", description("true resolves the comment, false reopens it")),
	)
}

func toolDeleteComment() *mcpgo.Tool {
	return newTool("delete_comment",
		withDescription("Delete a comment."),
		withString("id", required(), description(commentAddressDescription)),
		withString("comment_id", required(), description("Comment ID, as returned by list_comments")),
	)
}

func toolAcceptCommentSuggestion() *mcpgo.Tool {
	return newTool("accept_comment_suggestion",
		withDescription("Apply a comment's suggested replacement to the entity body and resolve the comment. "+
			"Needs permission to update the entity. Result: {content, warnings}."),
		withString("id", required(), description(commentAddressDescription)),
		withString("comment_id", required(), description("Comment ID, as returned by list_comments")),
	)
}

// commentOpResult answers a failed comment operation.
func commentOpResult(err error) *mcpgo.CallToolResult {
	var opErr *comments.OpError
	if errors.As(err, &opErr) {
		return errorResult(opErr.Error())
	}
	return errorResult("comment operation failed: " + err.Error())
}

// jsonResult marshals v as the tool's text result.
func jsonResult(v any) *mcpgo.CallToolResult {
	text, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errorResult(err.Error())
	}
	return textResult(string(text))
}

// commentTarget reads the id and, when commentID is set, the comment_id
// argument.
func commentTarget(args toolRequest, withCommentID bool) (address, commentID string, err error) {
	if address, err = args.RequireString("id"); err != nil {
		return "", "", err
	}
	address = trimID(address)
	if !withCommentID {
		return address, "", nil
	}
	if commentID, err = args.RequireString("comment_id"); err != nil {
		return "", "", err
	}
	commentID = strings.TrimSpace(commentID)
	if commentID == "" {
		return "", "", errors.New("comment_id must not be empty")
	}
	return address, commentID, nil
}

// optionalString reads a string argument that may be absent; nil means
// absent. A present value of another type is an error: falling back to
// "absent" would turn a mistyped replacement into no suggestion, and a
// mistyped body into no edit.
func optionalString(args toolRequest, key string) (*string, error) {
	if _, present := args.GetArguments()[key]; !present {
		return nil, nil //nolint:nilnil // nil is the absent argument, not a failure
	}
	v, err := args.RequireString(key)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// optionalBool is [optionalString] for a bool argument.
func optionalBool(args toolRequest, key string) (*bool, error) {
	v, present := args.GetArguments()[key]
	if !present {
		return nil, nil //nolint:nilnil // nil is the absent argument, not a failure
	}
	b, ok := v.(bool)
	if !ok {
		return nil, fmt.Errorf("argument %q is not a boolean", key)
	}
	return &b, nil
}

func (h commentHandler) handleListComments(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	address, _, err := commentTarget(newToolRequest(request), false)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	list, err := h.reader.ListComments(ctx, address)
	if err != nil {
		return commentOpResult(err), nil
	}
	return jsonResult(struct {
		Comments []comments.View `json:"comments"`
	}{Comments: list}), nil
}

func (h commentHandler) handleAddComment(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	args := newToolRequest(request)
	address, _, err := commentTarget(args, false)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	body, err := args.RequireString("body")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	kind, err := args.RequireString("anchor")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	in := comments.AddInput{Kind: comments.AnchorKind(kind), Body: body}
	for key, dst := range map[string]*string{
		"ref": &in.Ref, "quote": &in.Quote, "quote_prefix": &in.QuotePrefix, "quote_suffix": &in.QuoteSuffix,
	} {
		v, argErr := optionalString(args, key)
		if argErr != nil {
			return errorResult(argErr.Error()), nil
		}
		if v != nil {
			*dst = *v
		}
	}
	if in.Replacement, err = optionalString(args, "replacement"); err != nil {
		return errorResult(err.Error()), nil
	}
	created, err := h.writer.AddComment(ctx, address, in)
	if err != nil {
		return commentOpResult(err), nil
	}
	return jsonResult(created), nil
}

func (h commentHandler) handleUpdateComment(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	args := newToolRequest(request)
	address, commentID, err := commentTarget(args, true)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	body, err := optionalString(args, "body")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	resolved, err := optionalBool(args, "resolved")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	if body == nil && resolved == nil {
		return errorResult("nothing to update: pass body, resolved, or both"), nil
	}
	if err := h.writer.UpdateComment(ctx, address, commentID, body, resolved); err != nil {
		return commentOpResult(err), nil
	}
	return textResult("Updated comment " + commentID), nil
}

func (h commentHandler) handleDeleteComment(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	address, commentID, err := commentTarget(newToolRequest(request), true)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	if err := h.writer.DeleteComment(ctx, address, commentID); err != nil {
		return commentOpResult(err), nil
	}
	return textResult("Deleted comment " + commentID), nil
}

func (h commentHandler) handleAcceptSuggestion(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	address, commentID, err := commentTarget(newToolRequest(request), true)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	accepted, err := h.writer.AcceptSuggestion(ctx, address, commentID)
	if err != nil {
		return commentOpResult(err), nil
	}
	return jsonResult(accepted), nil
}
