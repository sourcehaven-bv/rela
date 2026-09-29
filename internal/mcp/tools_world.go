package mcp

import (
	"context"
	"log/slog"
	"sort"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// defaultWorldName is the reserved name of the world every entity's default
// state belongs to. It needs no grant.
const defaultWorldName = "default"

// worldArgDescription documents the `world` argument of the read tools.
const worldArgDescription = "World to read in (see list_worlds). A world picks one content state " +
	"per entity; `default` reads the default state. Omitted, the server's default world applies"

// WorldSelector binds a named world for the reads of one tool call. The
// remote server supplies it; it applies the same lookup and world grant as
// `?world=` on the data-entry API.
type WorldSelector interface {
	// SelectWorld returns ctx with name bound for every read on it. An
	// unknown world, or one the ctx principal may not read, is an error
	// whose message is safe to show the caller.
	SelectWorld(ctx context.Context, name string) (context.Context, error)
	// WorldReadable reports whether the ctx principal may select name.
	WorldReadable(ctx context.Context, name string) (bool, error)
	// DefaultWorld names the world a call that names none reads in.
	DefaultWorld() string
}

// selectWorld applies the tool call's `world` argument to ctx. It returns a
// non-nil result when the call must be refused.
func selectWorld(ctx context.Context, sel WorldSelector, args toolRequest) (context.Context, *mcpgo.CallToolResult) {
	name := args.GetString("world", "")
	if name == "" {
		return ctx, nil
	}
	if sel == nil {
		if name == defaultWorldName {
			return ctx, nil
		}
		return ctx, errorResult("this server does not resolve worlds; " +
			"read a content state as ID@face with show_entity")
	}
	bound, err := sel.SelectWorld(ctx, name)
	if err != nil {
		return ctx, errorResult(err.Error())
	}
	return bound, nil
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
	out := worldListJSON{
		DefaultWorld: defaultWorldName,
		Worlds:       []worldJSON{{Name: defaultWorldName, Readable: true}},
	}
	sel := deps.Worlds
	if sel == nil {
		out.Note = "this server does not resolve worlds; only `default` can be selected"
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

// otherFaces lists the faces of e, other than the one served, that st
// returns. st is the gated reader, so a face the caller's grant withholds is
// not listed: naming it would disclose that it exists. Each face is read by
// its explicit `ID@face` address, which no world re-resolves.
func otherFaces(ctx context.Context, st GraphReader, meta *metamodel.Metamodel, e *entity.Entity) []faceJSON {
	if meta == nil {
		return nil
	}
	def, ok := meta.GetEntityDef(e.Type)
	if !ok || len(def.Faces) == 0 {
		return nil
	}
	names := make([]string, 0, len(def.Faces))
	for name := range def.Faces {
		if name != e.Face.String() {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var out []faceJSON
	for _, name := range names {
		ref := e.ID + entity.StateRefSeparator + name
		if _, err := st.GetEntity(ctx, ref); err != nil {
			continue // absent, or not readable by the caller
		}
		out = append(out, faceJSON{Face: name, Label: faceLabel(meta, e.Type, name), Ref: ref})
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
