---
id: RR-HNKO3R
type: review-response
title: Scaffolder's typeLabel + "s" plural is itself an English derivation
finding: 'cmd/rela-desktop/main.go generateDataEntryConfig falls back to typeLabelPlural = typeLabel + "s" when LabelPlural is unset. That is strictly the same class of guess the decision removes: English-only, and wrong even in English (''Category'' -> ''Categorys''). It fires on the in-memory-metamodel path the fallback exists for.'
severity: minor
reason: Deferred deliberately, not overlooked. Three reasons. (1) It derives from an AUTHORED label (entDef.Label), not from an identifier — which is the line DEC-6C1NAA actually draws, and the same line that justifies keeping the metamodel relation-label strip. (2) It is scaffolder output written once into a file the user immediately edits, not a runtime rendering, so a wrong plural is visible and trivially correctable in a way a runtime guess is not. (3) The same Label+"s" fallback already exists in metamodel.EntityDef.GetLabelPlural and is used across the UI; changing the semantics of plurals is a broader change than this bug, and doing it here would leave the codebase inconsistent with GetLabelPlural rather than more consistent. Proper fix is to make LabelPlural authored-or-absent platform-wide (touching GetLabelPlural and its consumers) — worth its own ticket. The code comment at the site is honest about the fallback.
status: deferred
---
