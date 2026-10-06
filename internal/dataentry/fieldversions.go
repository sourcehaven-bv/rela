package dataentry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Per-field version tokens (TKT-2VDVHF).
//
// An autosave writes one field at a time. The entity ETag covers the whole
// row, so a client that sent it as If-Match would lose every save that
// followed an unrelated edit to another field. A token per field lets the
// server refuse only the write whose own field changed underneath it.
//
// Tokens are computed from the WIRE projection ([entitySerializer.forWireScoped]),
// never from the raw row, for two reasons. A redacted property is absent from
// the projection, so it gets no token: a token of a hidden value would let a
// caller confirm a guessed value by comparing hashes. And the GET that issues
// a token and the PATCH that checks it then hash exactly the same view of the
// row, whichever reader loaded it.
//
// A token is 8 hex characters (32 bits). It only has to tell one stored value
// from the value a client saw a moment ago, so a collision costs one missed
// conflict with probability 2^-32, which is below every other failure mode of
// an autosave.

// fieldToken hashes one field of one entity. id and type are part of the
// input so equal values on different entities get different tokens. The face
// is not: a precondition is only ever compared with the row the PATCH
// addresses, so equal values on two faces sharing a token is harmless.
// present distinguishes an unset property from one holding JSON null.
func fieldToken(id, typeName, field string, value any, present bool) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00", id, typeName, field)
	if !present {
		h.Write([]byte{0})
	} else {
		h.Write([]byte{1})
		h.Write(canonicalValue(value))
	}
	return hex.EncodeToString(h.Sum(nil)[:4])
}

// canonicalValue encodes a property value deterministically. encoding/json
// sorts map keys; a value it cannot encode falls back to its Go syntax,
// which is still deterministic for the scalar and container types a store
// returns.
func canonicalValue(value any) []byte {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Appendf(nil, "%#v", value)
	}
	return b
}

// Field names for the two non-property tokens. The NUL prefix keeps them
// apart from any property name, which cannot contain one.
const (
	contentTokenField   = "\x00content"
	relationsTokenField = "\x00relations"
)

// relationsToken hashes a wire relations map as a set: relation types and
// their targets are sorted, so the order a backend returns edges in does not
// change the token.
func relationsToken(id, typeName string, rels map[string][]string) string {
	types := make([]string, 0, len(rels))
	for t, targets := range rels {
		if len(targets) > 0 {
			types = append(types, t)
		}
	}
	sort.Strings(types)
	var b strings.Builder
	for _, t := range types {
		targets := slices.Clone(rels[t])
		sort.Strings(targets)
		b.WriteString(t)
		for _, to := range targets {
			b.WriteByte(0)
			b.WriteString(to)
		}
		b.WriteByte('\n')
	}
	return fieldToken(id, typeName, relationsTokenField, b.String(), true)
}

// fieldVersionsOf computes the tokens for a per-entity wire response. It
// covers every property present on the wire plus every declared property that
// is absent and not redacted, so a client can also state "I saw this empty".
// It must run after the serializer has stripped hidden properties and set
// Redacted.
func fieldVersionsOf(out *v1.Entity, meta *metamodel.Metamodel) *v1.FieldVersions {
	props := make(map[string]string, len(out.Properties))
	for k, v := range out.Properties {
		props[k] = fieldToken(out.ID, out.Type, k, v, true)
	}
	if def, ok := meta.Entities[out.Type]; ok {
		var redacted []string
		if out.Redacted != nil {
			redacted = *out.Redacted
		}
		for k := range def.Properties {
			if _, set := props[k]; set || slices.Contains(redacted, k) {
				continue
			}
			props[k] = fieldToken(out.ID, out.Type, k, nil, false)
		}
	}
	return &v1.FieldVersions{
		Properties: props,
		Content:    fieldToken(out.ID, out.Type, contentTokenField, out.Content, true),
		Relations:  relationsToken(out.ID, out.Type, out.Relations),
	}
}

// preconditionConflicts compares a PATCH's preconditions against the current
// tokens. A property the current projection has no token for is absent (the
// write already passed validateFieldWrite, so it is not hidden), and is
// compared against the absent token. Returns nil when every precondition
// holds.
func preconditionConflicts(pre *v1.Preconditions, cur *v1.FieldVersions, id, typeName string) *v1.FieldConflicts {
	var out v1.FieldConflicts
	found := false
	for k, expected := range pre.Properties {
		actual, ok := cur.Properties[k]
		if !ok {
			actual = fieldToken(id, typeName, k, nil, false)
		}
		if expected != actual {
			if out.Properties == nil {
				out.Properties = make(map[string]v1.Conflict)
			}
			out.Properties[k] = v1.Conflict{Expected: expected, Actual: actual}
			found = true
		}
	}
	if pre.Content != nil && *pre.Content != cur.Content {
		out.Content = &v1.Conflict{Expected: *pre.Content, Actual: cur.Content}
		found = true
	}
	if pre.Relations != nil && *pre.Relations != cur.Relations {
		out.Relations = &v1.Conflict{Expected: *pre.Relations, Actual: cur.Relations}
		found = true
	}
	if !found {
		return nil
	}
	return &out
}

// validatePreconditionScope reports the first precondition that names a
// field the PATCH does not write. A precondition on an unwritten field would
// turn the PATCH into a guard for a value it leaves alone, which no autosave
// channel needs and which would make a 412 ambiguous about what to retry.
func validatePreconditionScope(pre *v1.Preconditions, scope preconditionScope) (pointer string, ok bool) {
	for k := range pre.Properties {
		if _, set := scope.props[k]; set || slices.Contains(scope.unset, k) {
			continue
		}
		return "/preconditions/properties/" + v1.JSONPointerEscape(k), false
	}
	if pre.Content != nil && !scope.content {
		return "/preconditions/content", false
	}
	if pre.Relations != nil && !scope.relations {
		return "/preconditions/relations", false
	}
	return "", true
}
