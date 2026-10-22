package aclmap

import (
	"context"
	"errors"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// WebhookMapping is what one declarative webhook's data-entry.yaml mapping
// touches, in the terms the ACL decides on. The caller derives it from the
// config, so the engine keeps no data-entry dependency.
type WebhookMapping struct {
	// FindType and FindFace are the type and face the hook looks up. Empty
	// FindType: the hook has no find step.
	FindType string
	FindFace string
	// Match are the properties the find compares.
	Match []string
	// CreateType is the type create_if_missing creates; empty when the hook
	// never creates.
	CreateType string
	// Updates is true when the hook has `then` steps, which edit the found or
	// created entity.
	Updates bool
}

// WebhookNeed is one verb a hook's mapping needs, and whether the hook holds
// it type-wide (or on the face the mapping names).
type WebhookNeed struct {
	Verb string `json:"verb"`
	Type string `json:"type"`
	Face string `json:"face,omitempty"`
	Held bool   `json:"held"`
}

// WebhookCheck compares a hook's mapping with its effective access.
type WebhookCheck struct {
	// Needs lists each verb the mapping uses, in the order the delivery uses
	// them: read for the find, create, then update.
	Needs []WebhookNeed `json:"needs"`
	// HiddenMatchFields lists the find's match properties that the client
	// ceiling hides from the hook (BUG-KJ5D18). The read path redacts them,
	// so the find cannot compare them and never matches. Reported only when
	// the hook holds read on the find type; otherwise that need already
	// explains the failure.
	HiddenMatchFields []string `json:"hidden_match_fields,omitempty"`
}

// OK reports whether the mapping can run: every need is held and the find
// can see every match property.
func (c *WebhookCheck) OK() bool {
	if c == nil {
		return true
	}
	if len(c.HiddenMatchFields) > 0 {
		return false
	}
	for _, n := range c.Needs {
		if !n.Held {
			return false
		}
	}
	return true
}

// MapWebhook maps the principal a declarative webhook delivery runs as
// ([principal.Webhook]): the identity, tool and principal type the data-entry
// handler stamps. The report therefore applies the `applies_to: [webhook]`
// ceiling the runtime applies, without the caller naming the type.
//
// The name is used as is. The runtime does not resolve principal_property on
// `/hooks`, so neither does this report.
//
// mapping, when non-nil, adds a [WebhookCheck]. It is computed over the full
// verb set and the mapping's own types, whatever verbFilter and typeFilter
// narrow the rest of the report to. The engine does not check that hookID is
// configured; the caller owns data-entry.yaml.
func (e *Engine) MapWebhook(
	ctx context.Context, hookID string, mapping *WebhookMapping, verbFilter acl.Verb, typeFilter string,
	entityTypes []string, extras Extras,
) (*MapPrincipalResult, error) {
	hookID = strings.TrimSpace(hookID)
	if hookID == "" {
		return nil, errors.New("aclmap: webhook id must not be empty")
	}
	verbs, err := resolveVerbs(verbFilter)
	if err != nil {
		return nil, err
	}
	p := principal.Webhook(hookID)
	result := &MapPrincipalResult{
		SchemaVersion: schemaVersion,
		Principal:     p.User,
		As:            p.PrincipalType(),
		Webhook:       hookID,
	}
	result, err = e.mapFor(ctx, p, result, verbs, typeFilter, entityTypes, extras)
	if err != nil || mapping == nil {
		return result, err
	}
	req, err := e.requestFor(ctx, p)
	if err != nil {
		return nil, err
	}
	check, err := e.checkWebhook(ctx, req, *mapping)
	if err != nil {
		return nil, err
	}
	result.WebhookCheck = check
	return result, nil
}

// checkWebhook decides each need of mapping from the same sources the map
// reports: the type-wide baseline, or, for a need on a named face, the face
// grants alone. Per-entity exceptions do
// not count: a hook acts on whichever entity a delivery names, so a grant on
// some entities only is not a grant the hook can rely on.
func (e *Engine) checkWebhook(ctx context.Context, req *acl.Request, m WebhookMapping) (*WebhookCheck, error) {
	type need struct {
		verb      acl.Verb
		typ, face string
	}
	var needs []need
	if m.FindType != "" {
		needs = append(needs, need{acl.VerbRead, m.FindType, m.FindFace})
	}
	if m.CreateType != "" {
		face := ""
		if m.CreateType == m.FindType {
			face = m.FindFace
		}
		needs = append(needs, need{acl.VerbCreate, m.CreateType, face})
	}
	if m.Updates {
		for _, typ := range dedupeSorted([]string{m.FindType, m.CreateType}) {
			face := ""
			if typ == m.FindType {
				face = m.FindFace
			}
			needs = append(needs, need{acl.VerbUpdate, typ, face})
		}
	}

	check := &WebhookCheck{Needs: []WebhookNeed{}}
	for _, n := range needs {
		held, err := e.holdsNeed(ctx, req, n.verb, n.typ, n.face)
		if err != nil {
			return nil, err
		}
		check.Needs = append(check.Needs, WebhookNeed{
			Verb: string(n.verb), Type: n.typ, Face: n.face, Held: held,
		})
		if n.verb == acl.VerbRead && held {
			check.HiddenMatchFields = hiddenMatchFields(req, m)
		}
	}
	return check, nil
}

// hiddenMatchFields returns the match properties of the find that the client
// ceiling hides from the hook. Only the ceiling is checked. A role's `visible:`
// grant may depend on the entity, which a type-wide check cannot decide.
func hiddenMatchFields(req *acl.Request, m WebhookMapping) []string {
	ceiling := req.FieldCeilingFor(m.FindType)
	if !ceiling.Constrains() {
		return nil
	}
	var out []string
	for _, field := range dedupeSorted(m.Match) {
		if ceiling.Hides(field) {
			out = append(out, field)
		}
	}
	return out
}

// holdsNeed decides one need. A need on a named face is decided on that face
// only: the type-wide baseline counts bare and "*" write grants, which cover
// no face of a faced type at runtime.
func (e *Engine) holdsNeed(ctx context.Context, req *acl.Request, verb acl.Verb, typ, face string) (bool, error) {
	if face != "" {
		return e.heldOnFace(req, verb, typ, entity.Face(face)), nil
	}
	baseline, _, err := e.typeBaseline(ctx, req, typ, []acl.Verb{verb})
	if err != nil {
		return false, err
	}
	e.splitFaces(req, typ, []acl.Verb{verb}, baseline)
	_, held := baseline[string(verb)]
	return held, nil
}
