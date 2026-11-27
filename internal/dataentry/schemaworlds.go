package dataentry

import (
	"context"
	"log/slog"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// schemaWorlds builds the `worlds` block of `/api/v1/_schema`: every declared
// world, or the generated `default` world, each marked with whether THIS caller
// may select it. The declared set is the same for every caller, since world
// names are config. Only `readable` varies, and it is a UI hint: the server
// re-checks the grant on every request. Nothing here is per-entity, so it is no
// face-enumeration path. A gate error renders as not-readable. Slices are
// copied so the shared metamodel never reaches a response.
func schemaWorlds(ctx context.Context, meta *metamodel.Metamodel) map[string]v1.World {
	gate := readGateFromContext(ctx)

	defaultName := metamodel.EffectiveDefaultWorld(meta)
	out := map[string]v1.World{}
	if len(meta.Worlds) == 0 {
		out[defaultName] = v1.World{Readable: true, Default: true}
	}

	for name, def := range meta.Worlds {
		readable, err := name == defaultName, error(nil)
		if !readable {
			readable, err = gate.PermitsWorld(ctx, name)
		}
		if err != nil {
			slog.Warn("dataentry: schemaWorlds: PermitsWorld failed; reporting world as not readable",
				"world", name, "err", err)
			readable = false
		}
		out[name] = v1.World{
			Select:     append([]string(nil), def.Select...),
			Overrides:  worldOverrides(def),
			Otherwise:  string(def.Otherwise),
			PrimaryFor: append([]string(nil), def.PrimaryFor...),
			Banner:     def.Banner,
			Label:      def.Label,
			Hidden:     def.Hidden,
			Messages:   worldMessagesWire(def.Messages),
			OnAbsent:   worldOnAbsentWire(def.OnAbsent),
			Create:     def.Create,
			Readable:   readable,
			Default:    name == defaultName,
		}
	}
	return out
}

// worldOverrides copies a world's per-type chain overrides for the wire.
//
// Copied rather than handed over: the metamodel is a face shared by every
// assembled Services (appbuild.SharedBase), and a serializer that exposed its
// backing maps would let one response's consumer re-scope every reader's
// world. Nil for the common case of a world with no overrides, so the key is
// omitted entirely.
func worldOverrides(def metamodel.WorldDef) map[string][]string {
	if len(def.Overrides) == 0 {
		return nil
	}
	out := make(map[string][]string, len(def.Overrides))
	for typeName, chain := range def.Overrides {
		out[typeName] = append([]string(nil), chain...)
	}
	return out
}

// schemaFaceDefs renders one entity type's declared content-state
// coordinates for `/api/v1/_schema` (TKT-WRLDAPI item 3).
//
// Nil — hence an omitted key — for the common faceless type, keeping a
// project that declares no content states byte-identical on the wire.
//
// SCHEMA, not data: this reports what the operator declared for the TYPE.
// Which faces a given ENTITY holds is a different question, and deliberately
// not answerable here.
func schemaFaceDefs(def metamodel.EntityDef) map[string]v1.FaceDef {
	if len(def.Faces) == 0 {
		return nil
	}
	out := make(map[string]v1.FaceDef, len(def.Faces))
	for name, p := range def.Faces {
		out[name] = v1.FaceDef{Label: p.Label, Messages: faceMessagesWire(p.Messages)}
	}
	return out
}

// The operator's chrome text and behavior, projected verbatim. Each projector
// returns nil for an undeclared block so the key is omitted, not sent empty —
// a client treats absence and emptiness alike (render nothing), so the wire
// need not carry a block that says nothing.

func worldMessagesWire(m metamodel.WorldMessages) *v1.WorldMessages {
	if m == (metamodel.WorldMessages{}) {
		return nil
	}
	return &v1.WorldMessages{Absent: m.Absent, Projection: m.Projection, StandIn: m.StandIn}
}

func worldOnAbsentWire(o metamodel.WorldOnAbsent) *v1.WorldOnAbsent {
	if o.Redirect == "" {
		return nil
	}
	return &v1.WorldOnAbsent{Redirect: o.Redirect}
}

func faceMessagesWire(m metamodel.FaceMessages) *v1.FaceMessages {
	if m == (metamodel.FaceMessages{}) {
		return nil
	}
	return &v1.FaceMessages{ReadOnly: m.ReadOnly, Notice: m.Notice}
}
