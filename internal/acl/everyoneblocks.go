package acl

import (
	"fmt"
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
	// EntityType is the type the block is declared for.
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
		for _, block := range fieldBlocksOf(p.Roles[name]) {
			if !block.declaredBy(everyone) {
				out = append(out, block.openedBy(everyone, name, facesFor(facesOf, block.entityType))...)
			}
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

// fieldBlock is one field block a role declares for one entity type.
type fieldBlock struct {
	dimension  string
	entityType string
	// verbs are the verbs under which a role without this block gives the
	// whole dimension.
	verbs []Verb
}

// openedBy lists, per verb of b, the faces on which everyone holds the verb,
// as findings against the role named roleName.
func (b fieldBlock) openedBy(everyone RoleDef, roleName string, faces []entity.Face) []OpenedFieldBlock {
	var out []OpenedFieldBlock
	for _, verb := range b.verbs {
		var opened []entity.Face
		for _, face := range faces {
			if GrantsVerbOnFace(everyone, verb, b.entityType, face) {
				opened = append(opened, face)
			}
		}
		if len(opened) > 0 {
			out = append(out, OpenedFieldBlock{
				Role: roleName, Dimension: b.dimension, EntityType: b.entityType, Verb: verb, Faces: opened,
			})
		}
	}
	return out
}

// declaredBy reports whether role declares a block of the same dimension for
// the same type.
func (b fieldBlock) declaredBy(role RoleDef) bool {
	switch b.dimension {
	case "visible":
		_, ok := role.Visible[b.entityType]
		return ok
	case "fields":
		_, ok := role.Fields[b.entityType]
		return ok
	default:
		_, ok := role.Options[b.entityType]
		return ok
	}
}

// fieldBlocksOf lists the visible, fields and options blocks of role, sorted by
// entity type and then dimension.
func fieldBlocksOf(role RoleDef) []fieldBlock {
	writeVerbs := []Verb{VerbUpdate, VerbCreate}
	out := make([]fieldBlock, 0, len(role.Visible)+len(role.Fields)+len(role.Options))
	for t := range role.Visible {
		out = append(out, fieldBlock{dimension: "visible", entityType: t, verbs: []Verb{VerbRead}})
	}
	for t := range role.Fields {
		out = append(out, fieldBlock{dimension: "fields", entityType: t, verbs: writeVerbs})
	}
	for t := range role.Options {
		out = append(out, fieldBlock{dimension: "options", entityType: t, verbs: writeVerbs})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].entityType != out[j].entityType {
			return out[i].entityType < out[j].entityType
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
	}
	return fmt.Sprintf("role %q restricts %s on %q, but role %q holds %s on %q %s and declares no %s block; "+
		"field verdicts are the union of every role on the row and every principal holds %q, so %s the "+
		"restriction applies to nobody there",
		opened.Role, opened.Dimension, opened.EntityType, EveryoneRole, opened.Verb, opened.EntityType,
		where, opened.Dimension, EveryoneRole, operation)
}
