package acl

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// OpenedFieldBlock is a field block that restricts nobody on some faces,
// because the everyone role holds the block's verb there and declares no block
// of its own. Field verdicts are the union of every role on the row, a role
// without a block gives the whole dimension where it holds the verb
// (BUG-Y5UF3K), and every principal holds everyone.
type OpenedFieldBlock struct {
	// Role declares the block.
	Role string
	// Dimension is "visible", "fields" or "options".
	Dimension string
	// EntityType is the block key: the entity type, or `type@face` for a
	// block that names one face (TKT-RSVJKS).
	EntityType string
	// Verb is the verb everyone holds: read for visible, update for fields
	// and options on an existing row, create for them on a create.
	Verb Verb
	// Faces lists the faces on which everyone holds Verb, in the order
	// facesOf returned them. The unnamed face of a faceless type is "".
	Faces []entity.Face
}

// EveryoneOpenedFieldBlocks lists every block of another role that the
// everyone role opens. facesOf returns the declared faces of an entity type,
// or nil for a faceless type, whose rows live on the unnamed face. The answer
// is about the policy, not a principal, so no client ceiling applies: a
// ceiling may close the verb for some clients, but not for an unattenuated
// caller.
//
// Only everyone is considered. Every other role reaches a principal through
// assignments, membership edges or asserted claims, and whether every
// principal holds one is a property of the data, not of the policy.
//
// Nil: p accepted (no findings); facesOf accepted (every type is treated as
// faceless).
func EveryoneOpenedFieldBlocks(p *Policy, facesOf func(entityType string) []string) []OpenedFieldBlock {
	if p == nil {
		return nil
	}
	everyone, declared := p.Roles[EveryoneRole]
	if !declared {
		return nil
	}
	names := make([]string, 0, len(p.Roles))
	for name := range p.Roles {
		if name != EveryoneRole {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var out []OpenedFieldBlock
	for _, name := range names {
		role := p.Roles[name]
		for _, block := range fieldBlocksOf(role) {
			faces := []entity.Face{block.face}
			if block.face == "" {
				// A face the role also keys a block of this dimension for is
				// reported once, against that face block.
				faces = slices.DeleteFunc(facesFor(facesOf, block.entityType), func(face entity.Face) bool {
					return face != "" && block.hasKey(role, block.entityType+entity.StateRefSeparator+string(face))
				})
			}
			out = append(out, block.openedBy(everyone, name, faces)...)
		}
	}
	return out
}

// facesFor returns the faces of entityType as facesOf reports them, or the
// unnamed face alone for a faceless type or a nil facesOf.
func facesFor(facesOf func(string) []string, entityType string) []entity.Face {
	if facesOf == nil {
		return []entity.Face{""}
	}
	names := facesOf(entityType)
	if len(names) == 0 {
		return []entity.Face{""}
	}
	faces := make([]entity.Face, len(names))
	for i, name := range names {
		faces[i] = entity.Face(name)
	}
	return faces
}

// fieldBlock is one field block a role declares for one entity type, or for
// one face of it.
type fieldBlock struct {
	dimension string
	// key is the block key as written; entityType and face are its parts.
	key        string
	entityType string
	face       entity.Face
	// verbs are the verbs under which a role without this block gives the
	// whole dimension.
	verbs []Verb
}

// openedBy lists, per verb of b, the faces on which everyone holds the verb
// and declares no block of the same dimension, as findings against the role
// named roleName.
func (b fieldBlock) openedBy(everyone RoleDef, roleName string, faces []entity.Face) []OpenedFieldBlock {
	var out []OpenedFieldBlock
	for _, verb := range b.verbs {
		var opened []entity.Face
		for _, face := range faces {
			if !b.declaredBy(everyone, face) && GrantsVerbOnFace(everyone, verb, b.entityType, face) {
				opened = append(opened, face)
			}
		}
		if len(opened) > 0 {
			out = append(out, OpenedFieldBlock{
				Role: roleName, Dimension: b.dimension, EntityType: b.key, Verb: verb, Faces: opened,
			})
		}
	}
	return out
}

// declaredBy reports whether role declares a block of the same dimension
// that applies on face of the type: the bare type key, or the key naming
// face.
func (b fieldBlock) declaredBy(role RoleDef, face entity.Face) bool {
	keys := []string{b.entityType}
	if face != "" {
		keys = append(keys, b.entityType+entity.StateRefSeparator+string(face))
	}
	return slices.ContainsFunc(keys, func(key string) bool { return b.hasKey(role, key) })
}

// hasKey reports whether role declares a block of b's dimension under key.
func (b fieldBlock) hasKey(role RoleDef, key string) bool {
	var ok bool
	switch b.dimension {
	case "visible":
		_, ok = role.Visible[key]
	case "fields":
		_, ok = role.Fields[key]
	default:
		_, ok = role.Options[key]
	}
	return ok
}

// fieldBlocksOf lists the visible, fields and options blocks of role, sorted by
// entity type and then dimension.
func fieldBlocksOf(role RoleDef) []fieldBlock {
	writeVerbs := []Verb{VerbUpdate, VerbCreate}
	out := make([]fieldBlock, 0, len(role.Visible)+len(role.Fields)+len(role.Options))
	// A key whose face does not parse is refused when the affordance resolver
	// loads the policy, so it opens nothing and is skipped here.
	add := func(dimension, key string, verbs []Verb) {
		entityType, faceName, faced := strings.Cut(key, entity.StateRefSeparator)
		var face entity.Face
		if faced {
			parsed, err := entity.ParseFace(faceName)
			if err != nil || parsed == "" || strings.Contains(faceName, entity.StateRefSeparator) {
				return
			}
			face = parsed
		}
		out = append(out, fieldBlock{dimension: dimension, key: key, entityType: entityType, face: face, verbs: verbs})
	}
	for t := range role.Visible {
		add("visible", t, []Verb{VerbRead})
	}
	for t := range role.Fields {
		add("fields", t, writeVerbs)
	}
	for t := range role.Options {
		add("options", t, writeVerbs)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].key != out[j].key {
			return out[i].key < out[j].key
		}
		return out[i].dimension < out[j].dimension
	})
	return out
}

// Explain says in one sentence why opened restricts nobody, for the audit
// finding and the load-time warning alike.
func (opened OpenedFieldBlock) Explain() string {
	where := "on every row"
	if len(opened.Faces) > 1 || opened.Faces[0] != "" {
		names := make([]string, len(opened.Faces))
		for i, face := range opened.Faces {
			names[i] = string(face)
			if face == "" {
				names[i] = "(unnamed)"
			}
		}
		where = "on faces " + strings.Join(names, ", ")
	}
	var operation string
	switch opened.Verb {
	case VerbRead:
		operation = "on a read"
	case VerbCreate:
		operation = "on a create"
	case VerbUpdate, VerbDelete:
		operation = "on an update"
	case VerbRename:
		// Not reached: field blocks open only on read, create and update.
		operation = "on a rename"
	case VerbConvert:
		operation = "on a conversion"
	}
	return fmt.Sprintf("role %q restricts %s on %q, but role %q holds %s on %q %s and declares no %s block; "+
		"field verdicts are the union of every role on the row and every principal holds %q, so %s the "+
		"restriction applies to nobody there",
		opened.Role, opened.Dimension, opened.EntityType, EveryoneRole, opened.Verb, grantTypeOf(opened.EntityType),
		where, opened.Dimension, EveryoneRole, operation)
}
