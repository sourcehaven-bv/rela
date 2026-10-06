// coverage-ignore: MCP tool handlers - tested via integration tests
package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

func (s *Server) handleListRelations(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	snap := s.state.current()
	args := newToolRequest(request)
	relType := strings.TrimSpace(args.GetString("type", ""))
	from := trimID(args.GetString("from", ""))
	to := trimID(args.GetString("to", ""))
	limit := limitArg(args, defaultListLimit)
	offset := args.GetInt("offset", 0)

	d := snap.deps
	if relType != "" {
		if _, ok := d.Meta.GetRelationDef(relType); !ok {
			return errorResult("unknown relation type: " + relType), nil
		}
	}
	q := store.RelationQuery{Type: relType, From: from, To: to}

	all := make([]*entity.Relation, 0)
	for r, err := range d.Store.ListRelations(ctx, q) {
		if err != nil {
			return errorResult(err.Error()), nil
		}
		all = append(all, r)
	}

	sortStoreRelations(all)
	total := len(all)
	all = applyPagination(all, offset, limit)

	text, err := marshalJSON(struct {
		Total     int            `json:"total"`
		HasMore   bool           `json:"has_more"`
		Relations []relationJSON `json:"relations"`
	}{total, max(offset, 0)+len(all) < total, convertStoreRelationsList(all)})
	if err != nil { // coverage-ignore: defensive: relation DTOs of strings and YAML-derived maps; cannot fail.
		return errorResult(err.Error()), nil
	}
	return textResult(text), nil
}

func (s *Server) handleCreateRelation(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	snap := s.state.current()
	args := newToolRequest(request)
	fromID, err := args.RequireString("from")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	fromID = trimID(fromID)
	relType, err := args.RequireString("type")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	relType = strings.TrimSpace(relType)
	toID, err := args.RequireString("to")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	toID = trimID(toID)

	// Treat an empty `content` string from the MCP request as "leave alone"
	// rather than "set body to empty". MCP clients can omit the field or
	// pass null to mean the same; an explicit "" today never reaches a
	// no-content-meant-empty case in practice.
	// A relation's target has no face; refuse `ID@face` rather than let the
	// store reject it as a malformed id after authorization.
	if strings.Contains(toID, entity.StateRefSeparator) {
		return errorResult("to must be a bare entity id: a relation's target has no face"), nil
	}
	// Gate both endpoints first: the write path answers differently for a
	// hidden entity and a missing one, which would confirm it exists.
	for _, id := range []string{fromID, toID} {
		if !readable(ctx, snap.deps.Store, id) {
			return errorResult("entity not found: " + id), nil
		}
	}
	from, tailErr := relationTail(ctx, snap.deps.Store, snap.deps.Meta, fromID, relType)
	if tailErr != nil {
		return errorResult(tailErr.Error()), nil
	}

	key := entity.RelationKey{From: from.ID, FromFace: from.Face, Type: relType, To: toID}
	opts := entity.RelationOptions{
		Properties: extractProperties(request),
		Content:    nilIfEmpty(args.GetString("content", "")),
	}

	if _, createErr := snap.deps.EntityManager.CreateRelation(ctx, key, opts); createErr != nil {
		return errorResult(createErr.Error()), nil
	}

	return textResult(
		fmt.Sprintf("Created link: %s --%s--> %s", fromID, relType, toID)), nil
}

func (s *Server) handleDeleteRelation(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	snap := s.state.current()
	args := newToolRequest(request)
	fromID, err := args.RequireString("from")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	fromID = trimID(fromID)
	relType, err := args.RequireString("type")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	relType = strings.TrimSpace(relType)
	toID, err := args.RequireString("to")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	toID = trimID(toID)

	from, tailErr := relationTail(ctx, snap.deps.Store, snap.deps.Meta, fromID, relType)
	if amb, ok := errors.AsType[*visibility.AmbiguousAddressError](tailErr); ok {
		return errorResult(amb.Error()), nil
	}
	if tailErr != nil || !edgeVisible(ctx, snap.deps.Store, from, relType, toID) {
		return errorResult(
			fmt.Sprintf("relation not found: %s --%s--> %s", fromID, relType, toID)), nil
	}

	delErr := snap.deps.EntityManager.DeleteRelation(ctx,
		entity.RelationKey{From: from.ID, FromFace: from.Face, Type: relType, To: toID})
	if delErr != nil {
		return errorResult(delErr.Error()), nil
	}

	return textResult(
		fmt.Sprintf("Removed link: %s --%s--> %s", fromID, relType, toID)), nil
}

// relationTail is the tail an edge of relType from addr hangs on; the tail
// is part of the edge's identity (BUG-J3PBFN). A content-scoped edge
// belongs to one face: `ID@face` names it, and a bare id resolves to it as
// a write does ([GraphReader.WriteTarget]), so a miss is "entity not
// found" and an ambiguous id names its faces. An identity-scoped edge
// hangs on the entity, at the implicit face; a named face there is kept so
// the write path refuses it.
func relationTail(
	ctx context.Context, st GraphReader, meta *metamodel.Metamodel, addr, relType string,
) (entity.Ref, error) {
	if metamodel.IsContentScoped(meta, relType) {
		ref, err := st.WriteTarget(ctx, addr)
		if _, ok := errors.AsType[*visibility.AmbiguousAddressError](err); ok {
			return entity.Ref{}, err
		}
		if err != nil {
			return entity.Ref{}, errors.New("entity not found: " + addr)
		}
		return ref, nil
	}
	ref, err := entity.ParseRef(addr)
	if err != nil {
		return entity.Ref{}, errors.New("entity not found: " + addr)
	}
	return ref, nil
}

// edgeVisible reports whether the gated store serves the edge tailed at from.
func edgeVisible(ctx context.Context, st GraphReader, from entity.Ref, relType, to string) bool {
	face := from.Face
	q := store.RelationQuery{From: from.ID, FromFace: &face, Type: relType, To: to}
	for rel, err := range st.ListRelations(ctx, q) {
		if err != nil {
			return false
		}
		if rel.From == from.ID && rel.FromFace == face && rel.Type == relType && rel.To == to {
			return true
		}
	}
	return false
}

// nilIfEmpty returns nil when s is empty, else &s. Used to translate
// "absent / empty string" inputs from the MCP layer into the
// leave-alone semantic of entity.RelationOptions.Content.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
