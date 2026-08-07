---
id: DEC-6C1NAA
type: decision
title: 'Labels are authored, never derived: remove titleCase auto-labelling platform-wide'
context: |-
    The data-entry cleanup migration stripped any form label equal to titleCase(property), on the assumption the SPA would re-derive it. The SPA falls back to the raw identifier instead, so labels were permanently downgraded (BUG-8N2WT2) — and because the server refuses to start on unmigrated config, removing the label was mandatory and the degradation unavoidable.

    The obvious fix was to teach the SPA the same titleCase transform. That was rejected. titleCase is an English orthographic convention (replace _/- with spaces, upper-case each word's first rune) hardcoded into a platform whose metamodel is explicitly language-neutral. It is right by coincidence for a subset of Latin-script languages and silently wrong elsewhere: it has no notion of Dutch/German capitalization rules, and unicode.ToUpper on a first rune is not correct title-casing for many scripts. The reporting project is Dutch.

    The transform was also duplicated 11 times (4 in Go, 7 in the frontend) and the two families had already diverged on kebab-case — Go yields 'Kebab Name', the JS copies yield 'Kebab-Name'. Teaching the SPA the transform would have welded that duplication into a pinned cross-language invariant, making it harder to remove later.
consequences: |-
    A label is a human-authored, localized string. `label:` is the ONLY source of a label; when absent, the raw identifier is displayed. Applied with NO exceptions — no component in the system guesses a label from an identifier, at runtime or at generation time.

    1. titleCase auto-labelling removed from internal/dataentry/sections.go:184,226 (server-rendered view sections). Deliberate visible regression: unlabelled fields go from 'Laatste Contact' to 'laatste_contact'.

    2. isRedundantLabel deleted, and the titleCase arm of isRedundantRelationLabel with it. The migration no longer strips labels it cannot prove are re-derived, removing the forced-startup-failure trap.

    3. The metamodel arm of isRedundantRelationLabel is KEPT — a relation label equal to metamodel.yaml's RelationDef.Label is genuinely re-derivable: server-authored, already on the wire (types/schema.ts:37), language-neutral. RelationPicker/RelationCards fixed to consult relationType.label, making `label:` in metamodel.yaml live config for forms rather than dead config. This is the one sanctioned derivation and it derives from an AUTHORED label, never from an identifier.

    4. internal/lua/flow.go:374-379 stops defaulting a flow form field's label to titleCase(field.Name). The sibling parseMarkdownField path already does not derive.

    5. cmd/rela-desktop/main.go:829-853 (generateDataEntryConfig) stops WRITING titleCase labels into scaffolded data-entry.yaml. Even though those are explicit editable labels, emitting them bakes an English guess into new user projects. Scaffolded configs now carry no label: keys; the user authors every label in their own language.

    6. metamodel InverseDef.GetLabel() (types.go:579) stops deriving 'addressedBy' -> 'addressed by' via camelCaseToSpaced, and migration/inverse_simplify.go:91 stops stripping inverse labels matching that derivation. Same bug pattern one layer down; included so no half-applied rule remains.

    7. General rule for future migrations: strip only metamodel-grounded redundancy, where the server re-derives the value and the contract is verifiable. Never strip convention-grounded redundancy that depends on a client re-implementing a transform. A migration may only delete config it can prove is re-derived, and that proof belongs in a test.

    Rejected alternative: a one-time migration WRITING label: titleCase(prop) into existing data-entry.yaml to preserve appearance. It would avoid the regression (89/94 form fields and 41/41 list columns in tickets/data-entry.yaml are unlabelled) and make config explicit, but bakes an English guess into user files — the same objection that rules out the rela-desktop generator. Enum VALUE display labels (FEAT-JIBWQP's `labels:` map) are a SEPARATE explicit mechanism that never used titleCase and are unaffected.
date: "2026-08-06"
status: accepted
---

## Context

See BUG-8N2WT2 for the full reproduction. In short: `rela migrate` deleted
`label: 'Titel'` from a field for property `titel`, the SPA rendered `titel`,
and the server refused to boot if the label was left in place.

## The rejected fix

The narrow fix is to make `FieldRenderer.vue:42` apply the same `titleCase` the
migration assumes. It is a two-line change and it works. It was rejected because
it entrenches the wrong thing.

`titleCase` encodes an English orthographic convention:

```go
// internal/migration/dataentry_cleanup.go:443
func titleCase(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	words := strings.Fields(s)
	for i, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}
```

`strings.Fields` assumes words are space-separated; `unicode.ToUpper` on the
first rune assumes a bicameral script where initial-capital is the display
convention. Neither holds generally. rela's metamodel is language-neutral by
design — the entity types, property names, and labels are whatever the user
writes, in whatever language. Deriving presentation from an identifier smuggles
an English assumption into a schema-driven platform.

It works for Dutch by luck (shared ASCII word-boundary convention). It would not
work for a language with different capitalization rules, and it is meaningless
for a non-spaced script.

## Why not just fix the consumer

Because the contract would then need to be *enforced*, and enforcing it means
pinning eleven implementations to each other:

- Go: `dataentry/helpers.go:312`, `migration/dataentry_cleanup.go:443`,
`lua/flow.go:670`, `cmd/rela-desktop/main.go:895`
- Frontend: `FilterBar.vue:160`, `AdHocFilterMenu.vue:160`, `SearchView.vue:58`,
`RelationCards.vue:363`, `InlineCreateModal.vue:97`, plus inlined copies in
`DocumentsPanel.vue:145` and `DocumentView.vue:54`

These have already drifted: Go replaces `-`, the JS copies do not, so
`kebab-name` is stripped by Go as `Kebab Name` and would be re-derived by JS as
`Kebab-Name`. Verified empirically.

Building a round-trip test to hold eleven copies in agreement is a large
investment in protecting a heuristic that shouldn't exist.

## The rule

**A label is authored, not derived.** `label:` is the only source. Absent means
show the identifier.

This is simpler to explain, correct in every language, and makes the config
honest: if you want a pretty label, you write one, and it says what you wrote.

## Accepted cost

Server-rendered view sections regress visibly — `sections.go:184` and `:226`
currently fill an empty label with `titleCase(f.Property)`, and will stop. Users
relying on that auto-labelling will see raw identifiers until they add `label:`
entries. This was chosen deliberately over leaving one corner of the platform
clever; a half-removed heuristic is worse than either extreme, because the
behaviour then depends on which surface you're looking at.
