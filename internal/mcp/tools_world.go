package mcp

import (
	"context"
	"log/slog"
	"sort"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// worldArgDescription documents the `world` argument of the read tools.
const worldArgDescription = "World to read in (see list_worlds). A world picks one face (content state) " +
	"per entity. Omitted, the server's default world applies"

// WorldSelector resolves a named world for the reads of one tool call. The
// remote server supplies it; it applies the same lookup and world grant as
// `?world=` on the data-entry API.
type WorldSelector interface {
	// SelectWorld returns the world name selects. An unknown world, or one
	// the ctx principal may not read, is an error whose message is safe to
	// show the caller.
	SelectWorld(ctx context.Context, name string) (store.WorldScope, error)
	// WorldReadable reports whether the ctx principal may select name.
	WorldReadable(ctx context.Context, name string) (bool, error)
	// DefaultWorld names the world a call that names none reads in.
	DefaultWorld() string
}

// selectWorld applies the tool call's `world` argument. It returns the world
// the call reads in, ctx carrying it for bare-id reads
// ([visibility.WithReadWorld]), and a non-nil result when the call must be
// refused. With no argument the call reads in d.World and ctx is unchanged.
func selectWorld(
	ctx context.Context, d Deps, args toolRequest,
) (context.Context, store.WorldScope, *mcpgo.CallToolResult) {
	name := args.GetString("world", "")
	if name == "" {
		return ctx, d.World, nil
	}
	if d.Worlds == nil {
		// Without a selector the server reads in its default world only.
		if name == metamodel.EffectiveDefaultWorld(d.Meta) {
			return ctx, d.World, nil
		}
		return ctx, store.WorldScope{}, errorResult("this server does not resolve worlds; " +
			"read a face as ID@face with show_entity")
	}
	scope, err := d.Worlds.SelectWorld(ctx, name)
	if err != nil {
		return ctx, store.WorldScope{}, errorResult(err.Error())
	}
	return visibility.WithReadWorld(ctx, visibility.WorldOf(scope)), scope, nil
}

func toolListWorlds() *mcpgo.Tool {
	return newTool("list_worlds",
		withDescription("List the worlds the read tools accept as `world`. A world picks one content "+
			"state (face) per entity: `select` is the preferred order of faces, `overrides` replaces it "+
			"per entity type, and `otherwise` says what happens to an entity with none of them "+
			"(`exclude` or `default`). `readable` says whether you may select the world, and "+
			"`default_world` is the world used when a call names none"),
	)
}

// worldJSON describes one world for list_worlds. World definitions are
// operator config and are served to every caller; only `readable` depends on
// the caller.
type worldJSON struct {
	Name      string              `json:"name"`
	Readable  bool                `json:"readable"`
	Select    []string            `json:"select,omitempty"`
	Overrides map[string][]string `json:"overrides,omitempty"`
	Otherwise string              `json:"otherwise,omitempty"`
}

type worldListJSON struct {
	DefaultWorld string      `json:"default_world"`
	Worlds       []worldJSON `json:"worlds"`
	// Note explains a server that does not resolve worlds.
	Note string `json:"note,omitempty"`
}

// handleListWorlds serves list_worlds. A readability check that fails is
// logged and reported as not readable, so an outage never offers a world
// whose grant could not be confirmed.
func handleListWorlds(ctx context.Context, deps Deps) *mcpgo.CallToolResult {
	out := worldListJSON{DefaultWorld: metamodel.EffectiveDefaultWorld(deps.Meta)}
	if d := deps.Meta; d == nil || len(d.Worlds) == 0 {
		// The generated world, which needs no grant.
		out.Worlds = []worldJSON{{Name: out.DefaultWorld, Readable: true}}
	}
	sel := deps.Worlds
	if sel == nil {
		out.Note = "this server does not resolve worlds; only `" + out.DefaultWorld + "` can be selected"
	} else {
		out.DefaultWorld = sel.DefaultWorld()
	}
	var declared map[string]metamodel.WorldDef
	if deps.Meta != nil {
		declared = deps.Meta.Worlds
	}
	names := make([]string, 0, len(declared))
	for name := range declared {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		def := declared[name]
		readable := false
		if sel != nil {
			ok, err := sel.WorldReadable(ctx, name)
			if err != nil {
				slog.Warn("mcp: list_worlds: world grant check failed; reporting not readable",
					"world", name, "err", err)
			}
			readable = ok && err == nil
		}
		out.Worlds = append(out.Worlds, worldJSON{
			Name:      name,
			Readable:  readable,
			Select:    append([]string(nil), def.Select...),
			Overrides: copyOverrides(def.Overrides),
			Otherwise: string(def.Otherwise),
		})
	}
	text, err := marshalJSON(out)
	if err != nil { // coverage-ignore: defensive: out holds only strings, bools and string slices.
		return errorResult(err.Error())
	}
	return textResult(text)
}

// copyOverrides copies a world's per-type chains, so the shared metamodel's
// maps never reach a response.
func copyOverrides(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for typeName, chain := range in {
		out[typeName] = append([]string(nil), chain...)
	}
	return out
}

// otherFaces lists the faces of e, other than the one served, that the
// caller may read ([GraphReader.Family], headers only). A face the caller's
// grant withholds is not listed: naming it would disclose that it exists.
// Each comes with its explicit `ID@face` address, which no world re-resolves.
func otherFaces(ctx context.Context, st GraphReader, meta *metamodel.Metamodel, e *entity.Entity) []faceJSON {
	if meta == nil {
		return nil
	}
	fam, ok, err := st.Family(ctx, e.ID)
	if err != nil || !ok {
		return nil
	}
	var out []faceJSON
	for _, f := range fam.Faces {
		if f == e.Face || f.IsImplicit() {
			continue
		}
		ref := entity.FormatStateRef(e.ID, f)
		out = append(out, faceJSON{Face: f.String(), Label: faceLabel(meta, e.Type, f.String()), Ref: ref})
	}
	return out
}

// faceLabel is the operator's `label:` for a face, omitted when it only
// repeats the face name.
func faceLabel(meta *metamodel.Metamodel, entityType, face string) string {
	if label := metamodel.FaceLabel(meta, entityType, face); label != face {
		return label
	}
	return ""
}
