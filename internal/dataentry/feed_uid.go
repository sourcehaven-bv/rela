package dataentry

import "strings"

// feedUIDDomain is the fixed domain suffix for calendar event UIDs. It only
// needs to be stable (a changing UID makes calendar clients duplicate events),
// and the "<type>-<id>" local part is already unique within a rela instance, so
// a constant is sufficient and drift-proof (unlike a project-derived value).
const feedUIDDomain = "rela"

// feedUIDSep separates the entity type from the id in a UID's local part. A
// DOUBLE hyphen is used deliberately: entity ids reject "--" (it is the
// relation-key separator, see entity.ValidateID) and type names are
// single-hyphen kebab-case, so "--" appears in neither. That makes the split
// unambiguous even for hyphenated types like "test-case" or "review-response"
// (a single "-" separator would mis-split those).
const feedUIDSep = "--"

// feedUID builds a globally-unique, stable event UID from an entity's type and
// id: "<type>--<id>@rela". The type prefix is defensive (robust even for a
// metamodel that reuses id-space across types) and lets splitFeedUID route a
// CalDAV per-resource fetch back to the right source.
func feedUID(entityType, id string) string {
	return entityType + feedUIDSep + id + "@" + feedUIDDomain
}

// splitFeedUID reverses feedUID, returning the entity type and id. ok is false
// if s is not a UID this package minted (wrong domain, or missing the
// "<type>--<id>" shape).
func splitFeedUID(s string) (entityType, id string, ok bool) {
	local, domain, found := strings.Cut(s, "@")
	if !found || domain != feedUIDDomain {
		return "", "", false
	}
	entityType, id, found = strings.Cut(local, feedUIDSep)
	if !found || entityType == "" || id == "" {
		return "", "", false
	}
	return entityType, id, true
}
