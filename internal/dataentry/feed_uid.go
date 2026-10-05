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
// address: "<type>--<address>@rela". The address is the row's [entity.Ref]
// text, so a face of a faced type carries its face ("task--TSK-1@draft@rela")
// and a write through the UID or its href names the face it edits. When a
// world starts serving another face, the event's UID changes with it, which a
// client sees as one event removed and one added. The type prefix is defensive (robust even for a
// metamodel that reuses id-space across types) and lets splitFeedUID route a
// CalDAV per-resource fetch back to the right source.
func feedUID(entityType, addr string) string {
	return entityType + feedUIDSep + addr + "@" + feedUIDDomain
}

// splitFeedUID reverses feedUID, returning the entity type and address. The
// domain is cut at the LAST "@", since the address may carry a face. ok is false
// if s is not a UID this package minted (wrong domain, or missing the
// "<type>--<id>" shape).
func splitFeedUID(s string) (entityType, addr string, ok bool) {
	at := strings.LastIndex(s, "@")
	if at < 0 || s[at+1:] != feedUIDDomain {
		return "", "", false
	}
	entityType, addr, found := strings.Cut(s[:at], feedUIDSep)
	if !found || entityType == "" || addr == "" {
		return "", "", false
	}
	return entityType, addr, true
}
