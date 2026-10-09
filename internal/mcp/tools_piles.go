package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// maxPileIDs bounds the ids one add_to_pile or remove_from_pile call may
// name. It matches the size of a full pile.
const maxPileIDs = 500

// PileInfo is one of the acting user's piles. Items are newest first and
// include refs the caller can no longer read; the tools filter them through
// [Deps.Store].
type PileInfo struct {
	ID    string
	Name  string
	Icon  string
	Items []entity.Ref
}

// PilePush adds resolved refs to a pile by name, possibly another user's.
type PilePush struct {
	// Owner is the target user. Empty means the acting user.
	Owner  string
	Pile   string
	Refs   []entity.Ref
	Create bool
}

// PileReader reads the acting user's piles (the principal on ctx). Nothing
// here reads another user's pile.
type PileReader interface {
	ListPiles(ctx context.Context) ([]PileInfo, error)
	// FindPile returns the pile with that name, or with that pile id.
	FindPile(ctx context.Context, nameOrID string) (PileInfo, error)
}

// PileWriter changes piles.
type PileWriter interface {
	// PushToPile returns how many refs were new on the pile.
	PushToPile(ctx context.Context, p PilePush) (int, error)
	// RemoveFromPile drops refs from one of the acting user's piles, by id.
	RemoveFromPile(ctx context.Context, pileID string, refs []entity.Ref) error
}

// PileFuncs adapts plain functions to [PileReader] and [PileWriter], so the
// wiring site can supply the piles service as closures without this package
// importing it. Build one with [NewPileFuncs].
type PileFuncs struct {
	List   func(ctx context.Context) ([]PileInfo, error)
	Find   func(ctx context.Context, nameOrID string) (PileInfo, error)
	Push   func(ctx context.Context, p PilePush) (int, error)
	Remove func(ctx context.Context, pileID string, refs []entity.Ref) error
}

// NewPileFuncs returns f after checking it is complete.
//
// Nil: every function is required and a nil one is rejected.
func NewPileFuncs(f PileFuncs) (*PileFuncs, error) {
	if f.List == nil || f.Find == nil || f.Push == nil || f.Remove == nil {
		return nil, errors.New("mcp: NewPileFuncs requires List, Find, Push and Remove")
	}
	return &f, nil
}

// ListPiles serves [PileReader.ListPiles] through the injected List function.
func (f *PileFuncs) ListPiles(ctx context.Context) ([]PileInfo, error) { return f.List(ctx) }

// FindPile serves [PileReader.FindPile] through the injected Find function.
func (f *PileFuncs) FindPile(ctx context.Context, nameOrID string) (PileInfo, error) {
	return f.Find(ctx, nameOrID)
}

// PushToPile serves [PileWriter.PushToPile] through the injected Push function.
func (f *PileFuncs) PushToPile(ctx context.Context, p PilePush) (int, error) { return f.Push(ctx, p) }

// RemoveFromPile serves [PileWriter.RemoveFromPile] through the injected Remove function.
func (f *PileFuncs) RemoveFromPile(ctx context.Context, pileID string, refs []entity.Ref) error {
	return f.Remove(ctx, pileID, refs)
}

// WithPiles registers the pile tools (list_piles, show_pile, add_to_pile,
// remove_from_pile). Without it they are absent, which is right for a
// surface that has no per-user piles. Both halves are required; NewServer
// refuses one without the other.
func WithPiles(r PileReader, w PileWriter) Option {
	return func(s *Server) {
		s.pileReader = r
		s.pileWriter = w
		s.pilesSet = true
	}
}

// pileHandler serves the pile tools for one call. Built per call by
// [pilesFor] so it reads the current deps snapshot.
type pileHandler struct {
	reader PileReader
	writer PileWriter
	deps   Deps
}

// pilesFor returns the pile tool handler over the current snapshot. A free
// function to keep Server under its plimsoll load line.
func pilesFor(s *Server) pileHandler {
	return pileHandler{reader: s.pileReader, writer: s.pileWriter, deps: s.deps()}
}

// registerPileTools adds the pile tools when the server was built
// [WithPiles].
func registerPileTools(s *Server) {
	if s.pileReader == nil || s.pileWriter == nil {
		return
	}
	addTool(s, toolListPiles(), func(ctx context.Context, _ *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		return pilesFor(s).handleList(ctx), nil
	})
	addTool(s, toolShowPile(), func(ctx context.Context, req *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		return pilesFor(s).handleShow(ctx, newToolRequest(req)), nil
	})
	addTool(s, toolAddToPile(), func(ctx context.Context, req *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		return pilesFor(s).handleAdd(ctx, newToolRequest(req)), nil
	})
	addTool(s, toolRemoveFromPile(),
		func(ctx context.Context, req *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return pilesFor(s).handleRemove(ctx, newToolRequest(req)), nil
		})
}

func toolListPiles() *mcpgo.Tool {
	return newTool("list_piles",
		withDescription("List your piles: personal, named stacks of entities. "+
			"`count` is the number of items you can read. Result: {piles:[{id,name,icon,count}]}"),
	)
}

func toolShowPile() *mcpgo.Tool {
	return newTool("show_pile",
		withDescription("Show one of your piles: its items newest first, as {id,type,face,title,address}. "+
			"Items you cannot read are left out"),
		withString("pile", required(), description("Pile name or pile id")),
		withString("world", description(worldArgDescription)),
	)
}

func toolAddToPile() *mcpgo.Tool {
	return newTool("add_to_pile",
		withDescription("Add entities to a pile by name, on top. A full pile drops its oldest items. "+
			"Result: {added}. With owner set to another user, the push is write-only: added is always 0, "+
			"and a missing pile (with create false) or a user at the pile limit is a silent no-op"),
		withString("pile", required(), description("Pile name")),
		withArray("ids", required(), description("Entity IDs, or ID@face for one face of a faced type")),
		withString("owner", description("Person entity ID of the pile's owner (default: you)")),
		withBoolean("create", description("Create the pile when it does not exist (default true)")),
	)
}

func toolRemoveFromPile() *mcpgo.Tool {
	return newTool("remove_from_pile",
		withDescription("Remove entities from one of your piles. A bare ID removes every face of it; "+
			"ID@face removes that face. IDs not on the pile are ignored"),
		withString("pile", required(), description("Pile name or pile id")),
		withArray("ids", required(), description("Entity IDs, or ID@face")),
	)
}

type pileSummaryJSON struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Count int    `json:"count"`
}

type pileItemJSON struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Face    string `json:"face,omitempty"`
	Title   string `json:"title,omitempty"`
	Address string `json:"address"`
}

type pileJSON struct {
	pileSummaryJSON
	Items []pileItemJSON `json:"items"`
}

// handleList serves list_piles. Every pile's count comes from one batched
// header read over all their items.
func (h pileHandler) handleList(ctx context.Context) *mcpgo.CallToolResult {
	ps, err := h.reader.ListPiles(ctx)
	if err != nil {
		return errorResult(err.Error())
	}
	var all []entity.Ref
	for _, p := range ps {
		all = append(all, p.Items...)
	}
	resolved, err := h.deps.Store.ResolveHeadersErr(ctx, all)
	if err != nil {
		return pileReadFailed(err)
	}
	out := struct {
		Piles []pileSummaryJSON `json:"piles"`
	}{Piles: make([]pileSummaryJSON, 0, len(ps))}
	for _, p := range ps {
		count := 0
		for _, ref := range p.Items {
			if resolved[ref].Served() {
				count++
			}
		}
		out.Piles = append(out.Piles, pileSummaryJSON{ID: p.ID, Name: p.Name, Icon: p.Icon, Count: count})
	}
	return jsonResult(out)
}

// handleShow serves show_pile: the readable items, newest first, from one
// batched header read.
func (h pileHandler) handleShow(ctx context.Context, args toolRequest) *mcpgo.CallToolResult {
	name, err := args.RequireString("pile")
	if err != nil {
		return errorResult(err.Error())
	}
	ctx, _, refused := selectWorld(ctx, h.deps, args)
	if refused != nil {
		return refused
	}
	p, err := h.reader.FindPile(ctx, strings.TrimSpace(name))
	if err != nil {
		return errorResult(err.Error())
	}
	resolved, err := h.deps.Store.ResolveHeadersErr(ctx, p.Items)
	if err != nil {
		return pileReadFailed(err)
	}
	out := pileJSON{
		pileSummaryJSON: pileSummaryJSON{ID: p.ID, Name: p.Name, Icon: p.Icon},
		Items:           make([]pileItemJSON, 0, len(p.Items)),
	}
	for _, ref := range p.Items {
		rh, ok := resolved[ref]
		if !ok || !rh.Served() {
			continue
		}
		out.Items = append(out.Items, h.item(ref, rh))
	}
	out.Count = len(out.Items)
	return jsonResult(out)
}

// item is the wire form of one readable pile item.
func (h pileHandler) item(ref entity.Ref, rh visibility.ResolvedHeader) pileItemJSON {
	hd := rh.Header
	return pileItemJSON{
		ID:      hd.ID,
		Type:    hd.Type,
		Face:    hd.Face.String(),
		Title:   titleOrEmpty(hd.ID, h.deps.Meta.DisplayTitle(hd.ID, hd.Type, hd.Properties)),
		Address: ref.String(),
	}
}

// handleAdd serves add_to_pile. Each id resolves as update_entity's does
// ([GraphReader.WriteTarget]): it must be readable by the caller, and a bare
// id of a faced type is refused naming its readable faces. Nothing is pushed
// unless every id resolves (see [pileHandler.writeRefs]).
func (h pileHandler) handleAdd(ctx context.Context, args toolRequest) *mcpgo.CallToolResult {
	name, err := args.RequireString("pile")
	if err != nil {
		return errorResult(err.Error())
	}
	ids, bad := pileIDsArg(args)
	if bad != nil {
		return bad
	}
	refs, bad := h.writeRefs(ctx, ids)
	if bad != nil {
		return bad
	}
	added, err := h.writer.PushToPile(ctx, PilePush{
		Owner:  strings.TrimSpace(args.GetString("owner", "")),
		Pile:   name,
		Refs:   refs,
		Create: args.GetBool("create", true),
	})
	if err != nil {
		return errorResult(err.Error())
	}
	return jsonResult(struct {
		Added int `json:"added"`
	}{added})
}

// writeRefs resolves the ids of an add to the refs the pile stores, in
// order, as the HTTP add does. One batched header read settles every id it
// can: a named face the caller may read, and a bare id of a faceless type.
// Any other id of a readable family goes to [GraphReader.WriteTarget], which
// refuses a bare id of a faced type naming its faces. An id with no readable
// family is "entity not found" without a further read. A failed header read
// is a tool error.
func (h pileHandler) writeRefs(ctx context.Context, ids []string) ([]entity.Ref, *mcpgo.CallToolResult) {
	refs := make([]entity.Ref, len(ids))
	for i, id := range ids {
		ref, err := entity.ParseRef(id)
		if err != nil || ref.IsZero() {
			return nil, errorResult("entity not found: " + id)
		}
		refs[i] = ref
	}
	resolved, err := h.deps.Store.ResolveHeadersErr(ctx, refs)
	if err != nil {
		return nil, pileReadFailed(err)
	}
	for i, ref := range refs {
		res, found := resolved[ref]
		switch {
		case found && res.Served() && (!ref.Face.IsImplicit() || res.Header.Face.IsImplicit()):
			// A named face as named, or a faceless type's bare id.
		case found && res.Family:
			target, terr := h.deps.Store.WriteTarget(ctx, ids[i])
			if amb, ok := errors.AsType[*visibility.AmbiguousAddressError](terr); ok {
				return nil, errorResult(amb.Error())
			}
			if terr != nil {
				return nil, errorResult("entity not found: " + ids[i])
			}
			refs[i] = target
		default:
			return nil, errorResult("entity not found: " + ids[i])
		}
	}
	return refs, nil
}

// handleRemove serves remove_from_pile on one of the caller's own piles. It
// reads no graph, and its answer does not depend on which ids were on the
// pile, so removing a hidden, deleted or never-added entity looks the same.
func (h pileHandler) handleRemove(ctx context.Context, args toolRequest) *mcpgo.CallToolResult {
	name, err := args.RequireString("pile")
	if err != nil {
		return errorResult(err.Error())
	}
	ids, bad := pileIDsArg(args)
	if bad != nil {
		return bad
	}
	p, err := h.reader.FindPile(ctx, strings.TrimSpace(name))
	if err != nil {
		return errorResult(err.Error())
	}
	refs, err := pileItemsMatching(p.Items, ids)
	if err != nil {
		return errorResult(err.Error())
	}
	if len(refs) > 0 {
		if err := h.writer.RemoveFromPile(ctx, p.ID, refs); err != nil {
			return errorResult(err.Error())
		}
	}
	return textResult("Removed from pile " + p.Name)
}

// pileReadFailed answers a failed batched header read. It is logged and
// answered generically, so an outage is never mistaken for an empty pile or
// a missing entity.
func pileReadFailed(err error) *mcpgo.CallToolResult {
	slog.Warn("mcp: pile item read failed", "err", err)
	return errorResult("reading pile items failed")
}

// pileIDsArg reads the required `ids` array of addresses.
func pileIDsArg(args toolRequest) ([]string, *mcpgo.CallToolResult) {
	raw := args.GetStringSlice("ids", nil)
	ids := make([]string, 0, len(raw))
	for _, id := range raw {
		if id = trimID(id); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, errorResult("ids must name at least one entity")
	}
	if len(ids) > maxPileIDs {
		return nil, errorResult(fmt.Sprintf("at most %d ids per call, got %d", maxPileIDs, len(ids)))
	}
	return ids, nil
}

// pileItemsMatching maps addresses onto a pile's items: `ID@face` names that
// item, a bare id every item with that id.
func pileItemsMatching(items []entity.Ref, addrs []string) ([]entity.Ref, error) {
	var out []entity.Ref
	seen := make(map[entity.Ref]bool, len(addrs))
	for _, addr := range addrs {
		ref, err := entity.ParseRef(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid entity address %q", addr)
		}
		for _, it := range items {
			if (it == ref || (ref.Face.IsImplicit() && it.ID == ref.ID)) && !seen[it] {
				seen[it] = true
				out = append(out, it)
			}
		}
	}
	return out, nil
}

// jsonResult marshals v as the tool's text result.
func jsonResult(v any) *mcpgo.CallToolResult {
	text, err := marshalJSON(v)
	if err != nil { // coverage-ignore: defensive: the values are strings, ints and slices of them.
		return errorResult(err.Error())
	}
	return textResult(text)
}
