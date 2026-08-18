---
id: RR-4ICH8M
type: review-response
title: Section-level `render:` and unresolvable-source sections escape config validation
finding: 'The plan says ''enum validation in the two existing per-field loops (validate.go:912, 951)''. Both loops are nested inside source-resolution guards: `if sourceType != "" { if sourceDef, ok := meta.GetEntityDef(sourceType); ok { ... } } else if s.Source == "entry" { ... }`. A section whose source type has no entity def reaches neither loop, so `render: bogus` passes silently. Worse, `ViewSection.Render` — the section-level default the plan explicitly puts in scope — is validated by neither loop, since both iterate `s.Fields`. AC 5 compounds this by specifying an error naming ''view, section index, field index'', which has no meaning for a section-level value.'
severity: significant
resolution: 'Plan updated: `render` enum validation moves OUT of the source-resolution guards into an unconditional per-section pass that validates both `s.Render` and every `s.Fields[j].Render` regardless of whether the source type resolves. AC 5 split into 5a (field-level, names field index) and 5b (section-level, names section index only).'
status: addressed
---

Verified at `internal/dataentryconfig/validate.go:909-958`. The two field loops
sit inside source-resolution guards, so validation coverage depends on whether
the section's source type resolves to an entity def — which is orthogonal to
whether `render` is well-formed.

`render` needs no metamodel knowledge to validate (it is a closed enum), so it
should be checked unconditionally, before/outside the `sourceType` branching.
