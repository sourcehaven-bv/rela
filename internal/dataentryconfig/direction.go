package dataentryconfig

import (
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// DirectionResolution reports the outcome of inferring a relation binding's
// direction from the metamodel.
type DirectionResolution int

const (
	// DirectionResolved means the form's entity type sits on exactly one side
	// of the relation, so the direction has a single correct answer.
	DirectionResolved DirectionResolution = iota
	// DirectionAmbiguous means the entity type sits on BOTH sides (a
	// self-referencing relation such as `depends-on` from ticket to ticket).
	// Outgoing and incoming are both meaningful and mean opposite things, so
	// the author must say which one they meant.
	DirectionAmbiguous
	// DirectionNoSide means the entity type is on neither side. The binding is
	// wrong regardless of direction; validateFormRelationSide reports it with
	// the valid types.
	DirectionNoSide
	// DirectionUnknown means inference could not run at all: no metamodel, no
	// entity type, or a relation name absent from the metamodel. This is
	// deliberately distinct from DirectionNoSide — that one is a statement
	// ABOUT the relation, this one means we never got to look. A caller that
	// treats them alike will silently ship a default for a config it simply
	// failed to read.
	DirectionUnknown
)

// InferDirection resolves the direction of a relation binding authored without
// an explicit `direction:` key.
//
// Direction used to default to outgoing when absent, which is wrong in two
// different ways. On a `to`-side binding it silently bound the wrong side, and
// on a self-referencing relation it silently picked one of two equally valid
// readings. Inference removes the first case (there is only one right answer,
// so making the author write it adds nothing) and DirectionAmbiguous surfaces
// the second as an error rather than a guess.
//
// entityType is the form's entity type. A caller that does not know it (a
// metamodel-less migration pass) cannot infer and should not try.
//
// NOTE: internal/migration mirrors this truth table in
// FormRelationDirectionMigration.inferDirection. It cannot call this function —
// it walks yaml.Node against the narrower MetamodelProvider rather than a
// *metamodel.Metamodel — so the two must be changed in lockstep. If you add a
// resolution case here, add it there.
func InferDirection(
	entityType, relation string, meta *metamodel.Metamodel,
) (Direction, DirectionResolution) {
	if meta == nil || entityType == "" || relation == "" {
		return DirectionOutgoing, DirectionUnknown
	}
	def, ok := meta.GetRelationDef(relation)
	if !ok {
		return DirectionOutgoing, DirectionUnknown
	}
	inFrom := slices.Contains(def.From, entityType)
	inTo := slices.Contains(def.To, entityType)
	switch {
	case inFrom && inTo:
		return DirectionOutgoing, DirectionAmbiguous
	case inFrom:
		return DirectionOutgoing, DirectionResolved
	case inTo:
		return DirectionIncoming, DirectionResolved
	default:
		return DirectionOutgoing, DirectionNoSide
	}
}
