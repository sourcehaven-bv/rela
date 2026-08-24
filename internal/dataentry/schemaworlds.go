package dataentry

import (
	"context"
	"log/slog"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// schemaWorlds builds the `worlds` block of `/api/v1/_schema` (TKT-WRLDAPI
// item 1): every DECLARED world, plus the implicit `default` world, each
// marked with whether THIS caller may select it.
//
// # Why enumerate at all
//
// `?world=` selection has existed since TKT-DN37J2 with no way to discover a
// legal value: a client had to be told a name out of band or guess one, and a
// guess is answered by a 400. A world selector cannot be built against that.
// This is the enumeration `PermitsWorld` already answers per name.
//
// # The declared set is principal-independent; only `readable` is not
//
// Every declared world appears for every caller. World names are
// operator-authored `schema.yaml` config — routinely a public repo — so their
// existence is already disclosed, and CLAUDE.md is explicit that code must
// not filter a config surface per principal or contort to conceal a config
// name. Filtering here would also be the wrong shape for the UI: a selector
// that silently omits a world leaves the user unable to tell "no such world"
// from "not for you", which is worse than saying so.
//
// What stays secret is unchanged: a world's CONTENTS. Selecting a world this
// caller cannot read still yields an empty result rather than a 403
// (`errWorldDenied`), precisely so it is indistinguishable from a world
// holding nothing readable. `readable` is a UI hint about SELECTION, and the
// server re-checks the grant on every request regardless — a client ignoring
// this flag learns nothing it could not learn by asking.
//
// # This is not a face-enumeration path
//
// Nothing here is per-entity. A world's chain says which coordinates it would
// PREFER; it never says which faces any row actually holds. Two entities of
// the same type produce identical output whether one has a published face and
// the other does not — the existence oracle stays closed.
//
// A gate error is logged and rendered as not-readable: an outage must not
// widen a selector into offering a world whose grant could not be confirmed.
func schemaWorlds(ctx context.Context, meta *metamodel.Metamodel) map[string]v1.World {
	gate := readGateFromContext(ctx)

	// The default world is always present and always selectable. Emitted
	// explicitly rather than left implicit so a client need not hardcode the
	// reserved name to offer it.
	//
	// Readable is a CONSTANT true here, and deliberately not a gate call.
	// `resolveWorld` short-circuits `default` (and an absent parameter)
	// BEFORE any grant check, because the default world is today's graph and
	// gating it would gate the whole product. Asking the gate anyway would
	// make this enumeration contradict the server: acl.Request.PermitsWorld
	// has no default-world arm, so it answers false for `default` unless a
	// role happens to grant the `world:default` token — and the selector
	// would then report today's graph as unreadable while every request for
	// it succeeds.
	//
	// So this is not "trusting the default"; it is reporting the same
	// short-circuit the request path takes. If the request path ever starts
	// gating the default world, this must start asking too — the two are one
	// decision and must not drift.
	out := map[string]v1.World{
		defaultWorldName: {Readable: true, Default: true},
	}

	for name, def := range meta.Worlds {
		readable, err := gate.PermitsWorld(ctx, name)
		if err != nil {
			// Fail closed on an infrastructure failure. Reporting readable on
			// an unanswerable grant check is the fail-open direction: the
			// selector would offer a world whose requests then come back
			// empty, which reads as "nothing is published" rather than as the
			// outage it is. Logged loud so the operator sees the cause.
			slog.Warn("dataentry: schemaWorlds: PermitsWorld failed; reporting world as not readable",
				"world", name, "err", err)
			readable = false
		}
		out[name] = v1.World{
			// Copied, not aliased — see worldOverrides for why the shared
			// metamodel's slices must not reach a response.
			Select:    append([]string(nil), def.Select...),
			Overrides: worldOverrides(def),
			Otherwise: string(def.Otherwise),
			Readable:  readable,
		}
	}
	return out
}

// worldOverrides copies a world's per-type chain overrides for the wire.
//
// Copied rather than handed over: the metamodel is a pointer shared by every
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

// schemaPointers renders one entity type's declared content-state
// coordinates for `/api/v1/_schema` (TKT-WRLDAPI item 3).
//
// Nil — hence an omitted key — for the common pointerless type, keeping a
// project that declares no content states byte-identical on the wire.
//
// SCHEMA, not data: this reports what the operator declared for the TYPE.
// Which faces a given ENTITY holds is a different question, and deliberately
// not answerable here.
func schemaPointers(def metamodel.EntityDef) map[string]v1.Pointer {
	if len(def.Pointers) == 0 {
		return nil
	}
	out := make(map[string]v1.Pointer, len(def.Pointers))
	for name, p := range def.Pointers {
		out[name] = v1.Pointer{Default: p.Default}
	}
	return out
}
