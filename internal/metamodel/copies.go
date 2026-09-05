package metamodel

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// newEntityPrefix marks a copy's `to:` as CROSS-ENTITY — it creates the
// target rather than writing a face of the source (design doc §9.1).
const newEntityPrefix = "new "

// CopyTarget is a parsed `from:` / `to:` address.
type CopyTarget struct {
	// Type is the entity type. Always set on a valid target.
	Type string
	// Face is the content state, or "" for the default face.
	Face string
	// IsNew is true for the `new <type>` form, which creates the target.
	IsNew bool
}

// ParseCopyTarget splits a copy address: `type`, `type@face`, or
// `new type`. Whitespace-trimmed; the caller validates that the parts name
// declared things.
//
// The returned Face is the DECLARED NAME, which is not always the stored
// coordinate — a face marked `bare_face` IS the zero face
// (FaceDef.Default). Use [StoredFace] to resolve a declared
// name to the coordinate the store addresses.
func ParseCopyTarget(s string) (CopyTarget, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return CopyTarget{}, errors.New("empty target")
	}
	var t CopyTarget
	if rest, ok := strings.CutPrefix(s, newEntityPrefix); ok {
		t.IsNew = true
		s = strings.TrimSpace(rest)
		if s == "" {
			return CopyTarget{}, fmt.Errorf("%q names no entity type", newEntityPrefix)
		}
	}
	typeName, face, found := strings.Cut(s, "@")
	t.Type, t.Face = strings.TrimSpace(typeName), strings.TrimSpace(face)
	if t.Type == "" {
		return CopyTarget{}, errors.New("names a state but no entity type")
	}
	if found && t.Face == "" {
		return CopyTarget{}, fmt.Errorf("names no content state after %q", "@")
	}
	if t.IsNew && found {
		return CopyTarget{}, errors.New(
			"a `new` target creates an entity, so it addresses the type's default " +
				"state and must not name one")
	}
	return t, nil
}

// IsSameEntity reports whether a definition copies between faces of ONE
// entity (promote, revise) rather than into a different entity.
//
// This is the ELEVATION BOUNDARY (§9.2), so it is one predicate rather than
// a condition spelled out at each call site: same-entity copies run
// elevated, cross-entity copies read through the caller's visibility gate.
func (c CopyDef) IsSameEntity() bool {
	from, ferr := ParseCopyTarget(c.From)
	to, terr := ParseCopyTarget(c.To)
	if ferr != nil || terr != nil {
		return false
	}
	return !to.IsNew && from.Type == to.Type
}

// validateCopies checks the `copies:` block against the schema. Every check
// here is a LOAD-TIME refusal: a copy writes content into a face that a
// world may publish, so a definition that resolves wrongly is exactly the
// leak this feature exists to prevent.
func validateCopies(m *Metamodel) []string {
	if len(m.Copies) == 0 {
		return nil
	}
	names := make([]string, 0, len(m.Copies))
	for name := range m.Copies {
		names = append(names, name)
	}
	sort.Strings(names)

	var errs []string
	for _, name := range names {
		errs = append(errs, validateCopy(m, name, m.Copies[name])...)
	}
	return errs
}

func validateCopy(m *Metamodel, name string, def CopyDef) []string {
	var errs []string
	bad := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf("copy %q: "+format, append([]any{name}, args...)...))
	}
	if strings.TrimSpace(name) == "" {
		return []string{"copies: definition name must not be empty"}
	}

	from, err := ParseCopyTarget(def.From)
	if err != nil {
		bad("from: %v", err)
	}
	to, terr := ParseCopyTarget(def.To)
	if terr != nil {
		bad("to: %v", terr)
	}
	if err != nil || terr != nil {
		return errs
	}
	errs = append(errs, validateCopyEndpoint(m, name, "from", from)...)
	errs = append(errs, validateCopyEndpoint(m, name, "to", to)...)

	sameEntity := !to.IsNew && from.Type == to.Type
	if !sameEntity && def.AllFields {
		// The load error the whole cross-entity half turns on. A cross-entity
		// copy reads through the CALLER'S visibility gate, so `fields: all`
		// would write whatever survived redaction as if it were the whole
		// entity — destroying the fields the principal could not see. That is
		// the redacted read-modify-write the codebase forbids everywhere, and
		// refusing at load costs an operator a startup message instead of
		// silent field destruction at runtime.
		bad("`fields: all` is not allowed on a cross-entity copy — that copy " +
			"reads through the caller's visibility gate, so copying every field " +
			"would write a REDACTED entity and destroy the fields they cannot " +
			"see; name the fields to copy explicitly")
	}
	if def.AllFields && len(def.Fields) > 0 {
		bad("declares both `fields: all` and a field mapping — pick one")
	}
	if !def.AllFields && len(def.Fields) == 0 {
		bad("copies no fields — set `fields: all` (same-entity only) or map at least one")
	}
	if !sameEntity && from.Type != to.Type && len(def.Fields) == 0 {
		bad("a cross-type copy requires an explicit field map")
	}

	// A definition targeting a NON-DEFAULT face must carry a guard. §9.2's
	// "writable only by copy definitions naming it as target, EACH CARRYING
	// ITS OWN GUARD" is conditioned on the guard existing; an unguarded
	// definition into a guarded face would make the whole face writable by
	// anyone who can name the definition, which is the opposite of what
	// declaring it was supposed to buy.
	targetsGuardedFace := to.Face != "" && StoredFace(m, to.Type, to.Face) != ""
	if targetsGuardedFace && def.Guard.Permission == "" {
		bad("targets the guarded face %q@%q but declares no `guard: {permission: ...}` — "+
			"a guarded face is writable only through a definition that carries its "+
			"own guard, so an unguarded one would open the face to anyone who can "+
			"name this copy", to.Type, to.Face)
	}
	if def.Guard.When != "" {
		// Declared, parsed, and NOT evaluated. Accepting it silently would be
		// the worst outcome: an operator writes
		// `when: "source.status == 'approved'"`, the schema loads clean, and
		// unapproved drafts publish. Refusing costs them a startup message.
		bad("`guard.when` is not implemented yet — remove it, or the copy would " +
			"run without evaluating the condition you wrote")
	}

	errs = append(errs, validateCopyFields(m, name, def, to)...)
	errs = append(errs, validateCopyRelations(m, name, def, to)...)
	errs = append(errs, validateCopyLanding(m, name, def, to)...)
	return errs
}

// validateCopyLanding checks `on_success.landing`: a scalar must be `written`
// or `stay`; a world must be declared (or `default`); a face must be one the
// TARGET type declares, since the page lands on the entity the copy wrote.
// Naming both a world and a face is refused rather than resolved by a
// precedence rule nobody would remember.
func validateCopyLanding(m *Metamodel, name string, def CopyDef, to CopyTarget) []string {
	l := def.OnSuccess.Landing
	if l.IsZero() {
		return nil
	}
	bad := func(format string, args ...any) []string {
		return []string{fmt.Sprintf("copy %q: `on_success.landing` "+format, append([]any{name}, args...)...)}
	}
	if l.Mode != "" {
		if l.Mode != LandingWritten && l.Mode != LandingStay {
			return bad("must be %q, %q, `{world: <name>}` or `{face: <name>}` (got %q)",
				LandingWritten, LandingStay, l.Mode)
		}
		return nil
	}
	if l.World != "" && l.Face != "" {
		return bad("names both a world and a face — pick one")
	}
	if l.World != "" && l.World != DefaultWorldName {
		if _, ok := m.Worlds[l.World]; !ok {
			return bad("names world %q, which is not declared", l.World)
		}
	}
	if l.Face != "" {
		def, ok := m.Entities[to.Type]
		if !ok {
			return nil // the endpoint check already reported the type
		}
		if _, declared := def.Faces[l.Face]; !declared {
			return bad("names face %q, which type %q does not declare", l.Face, to.Type)
		}
	}
	return nil
}

// validateCopyEndpoint checks that a target names a declared type and, when
// it names a face, a face that type declares.
func validateCopyEndpoint(m *Metamodel, name, side string, t CopyTarget) []string {
	var errs []string
	def, ok := m.GetEntityDef(t.Type)
	if !ok {
		return []string{fmt.Sprintf(
			"copy %q: %s: entity type %q is not declared", name, side, t.Type)}
	}
	if t.Face == "" {
		return nil
	}
	if _, declared := def.Faces[t.Face]; !declared {
		errs = append(errs, fmt.Sprintf(
			"copy %q: %s: entity type %q declares no content state %q",
			name, side, t.Type, t.Face))
	}
	return errs
}

// validateCopyFields checks mapped field names against the TARGET type's
// declared properties. The value side is an interpolation template, checked
// only for grammar — its property references resolve at copy time.
func validateCopyFields(m *Metamodel, name string, def CopyDef, to CopyTarget) []string {
	targetDef, ok := m.GetEntityDef(to.Type)
	if !ok {
		return nil // already reported by validateCopyEndpoint
	}
	fields := make([]string, 0, len(def.Fields))
	for f := range def.Fields {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	var errs []string
	for _, f := range fields {
		if _, declared := targetDef.Properties[f]; !declared {
			errs = append(errs, fmt.Sprintf(
				"copy %q: fields: entity type %q declares no property %q",
				name, to.Type, f))
		}
	}
	return errs
}

// validateCopyRelations checks the per-relation-type verbs and that each
// named relation is declared and may originate from the target type.
func validateCopyRelations(m *Metamodel, name string, def CopyDef, to CopyTarget) []string {
	rels := make([]string, 0, len(def.Relations))
	for r := range def.Relations {
		rels = append(rels, r)
	}
	sort.Strings(rels)

	var errs []string
	for _, rel := range rels {
		switch def.Relations[rel] {
		case "merge", "replace":
		default:
			errs = append(errs, fmt.Sprintf(
				"copy %q: relations.%s: %q is not a valid mode (want \"merge\" or \"replace\")",
				name, rel, def.Relations[rel]))
		}
		relDef, ok := m.GetRelationDef(rel)
		if !ok {
			errs = append(errs, fmt.Sprintf(
				"copy %q: relations: relation type %q is not declared", name, rel))
			continue
		}
		// §9.2's FIRST mitigation for "copying relations can mint authority".
		// An identity-scoped edge attaches to the bare id and is shared by
		// every state (§2.2), so copying one onto a state tail is at best
		// meaningless and at worst mints a duplicate role-conferring edge
		// whose lifecycle nothing manages. Role-conferring types are declared
		// in acl.yaml, which the metamodel cannot see — but they are
		// overwhelmingly identity-scoped, so this catches the class.
		if relDef.Scope.IsIdentity() {
			errs = append(errs, fmt.Sprintf(
				"copy %q: relations.%s: relation %q is identity-scoped, so it attaches "+
					"to the entity rather than to a face and is shared by every state — "+
					"copying it would duplicate an edge that may confer roles; only "+
					"content-scoped relations can be copied",
				name, rel, rel))
			continue
		}
		if !slices.Contains(relDef.From, to.Type) {
			errs = append(errs, fmt.Sprintf(
				"copy %q: relations.%s: relation %q cannot originate from entity type %q "+
					"(its `from` is %v)", name, rel, rel, to.Type, relDef.From))
		}
	}
	return errs
}

// StoredFace resolves a DECLARED face name to the coordinate the store
// addresses for that face.
//
// A package function rather than a *Metamodel method: that type sits at its
// plimsoll exported-method cap, and the cap is a ratchet to narrow rather
// than raise.
//
// The mapping is identity except for the type's default face, which is
// stored under the ZERO coordinate: `bare_face` names which declared
// coordinate the default state answers to, it does not create a second row
// (design doc §2.1 — there are exactly N states and nothing else).
//
// Getting this wrong is not a cosmetic bug. A copy addressing `page@draft`
// when draft is the default would read a face that does not exist, and a
// copy WRITING it would mint a second row for a state the entity already
// has — two rows claiming to be the same face.
func StoredFace(m *Metamodel, entityType, declared string) string {
	if m == nil {
		return declared
	}
	if declared == "" {
		return ""
	}
	def, ok := m.GetEntityDef(entityType)
	if !ok {
		return declared
	}
	if _, found := def.Faces[declared]; found && declared == def.BareFace {
		return ""
	}
	return declared
}

// DeclaredFace is the inverse of [StoredFace]: it resolves a STORED
// coordinate back to the face name the operator declared.
//
// Identity except for the zero coordinate, which is the type's
// `bare_face` face when it declares one. A type with faces but no
// default has no declared name for its default state, and a type with no
// faces at all has no declared names whatsoever — both return "", which
// is the honest answer rather than an invented one.
//
// Display paths need this direction because the wire and the store speak
// stored coordinates while `faces:` (and therefore [FaceDef.Label]) is
// keyed by declared name.
func DeclaredFace(m *Metamodel, entityType, stored string) string {
	if stored != "" {
		return stored
	}
	if m == nil {
		return ""
	}
	def, ok := m.GetEntityDef(entityType)
	if !ok {
		return ""
	}
	// A direct read now that the bare face is named on the type: this used
	// to scan every face looking for the one flagged default, which also
	// meant the answer depended on map order if two ever claimed it.
	if _, ok := def.Faces[def.BareFace]; ok {
		return def.BareFace
	}
	return ""
}

// FaceLabel is the display text for one face of an entity type: the
// operator's [FaceDef.Label] when set, else the declared face name.
//
// Takes the STORED coordinate, because that is what every caller has in hand
// — the wire carries stored coordinates and so does the store. It resolves
// the declared name itself via [DeclaredFace].
//
// Returns "" only when there is no declared name to fall back to (an
// undeclared coordinate, or the default state of a type that declares no
// default face). Callers render their own last-resort word for that case;
// this function does not invent one, because "default" is a UI word and not
// a fact about the metamodel.
func FaceLabel(m *Metamodel, entityType, stored string) string {
	declared := DeclaredFace(m, entityType, stored)
	if declared == "" {
		return ""
	}
	if m == nil {
		return declared
	}
	def, ok := m.GetEntityDef(entityType)
	if !ok {
		return declared
	}
	if p, found := def.Faces[declared]; found && p.Label != "" {
		return p.Label
	}
	return declared
}
