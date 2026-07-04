package dataentry

import "strings"

// feedUIDDomain is the fixed domain suffix for calendar event UIDs. It only
// needs to be stable (a changing UID makes calendar clients duplicate events),
// and the "<type>-<id>" local part is already unique within a rela instance, so
// a constant is sufficient and drift-proof (unlike a project-derived value).
const feedUIDDomain = "rela"

// feedUID builds a globally-unique, stable event UID from an entity's type and
// id: "<type>-<id>@rela". The type prefix is defensive (robust even for a
// metamodel that reuses id-space across types) and lets splitFeedUID route a
// CalDAV per-resource fetch back to the right source.
func feedUID(entityType, id string) string {
	return entityType + "-" + id + "@" + feedUIDDomain
}

// splitFeedUID reverses feedUID, returning the entity type and id. ok is false
// if s is not a UID this package minted (wrong domain, or missing the
// "<type>-<id>" shape).
func splitFeedUID(s string) (entityType, id string, ok bool) {
	local, domain, found := strings.Cut(s, "@")
	if !found || domain != feedUIDDomain {
		return "", "", false
	}
	entityType, id, found = strings.Cut(local, "-")
	if !found || entityType == "" || id == "" {
		return "", "", false
	}
	return entityType, id, true
}
