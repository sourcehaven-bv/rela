---
id: RR-Z1V5Z6
type: review-response
title: The audit-log guide claims every automation has a name but nothing enforces it
finding: 'automations: is a YAML list where name is an ordinary optional field and no validator checks it'
severity: significant
resolution: 'Docs corrected. The guide no longer claims every automation has a name; it now states that name: is an optional field nothing rejects and that the bare label means the schema did not say which rule this was. Also added a paragraph documenting the one-label-per-record policy. Load-time validation was NOT added - see RR-2FY0O8''s sibling note; that is a metamodel-loader change beyond this ticket''s scope.'
status: addressed
---

The docs sentence added by this ticket — *"you should not normally see it, since
every automation declared in `schema.yaml` has a `name:`"* — states an invariant
nothing enforces.

Verified: `automations:` is a YAML **list** of structs (`- name: foo`), not a
map keyed by name. Entities, relations and types are `map[string]...` where the
key *is* the name and cannot be empty; automations are the one declaration form
where the name is an ordinary optional field.

`metamodel/loader.go`'s `validate()` calls eight validators (entity structure,
custom types, entity semantics, relation
references/properties/inverses/orderable, transforms). There is **no
`validateAutomations`** — the word "automation" appears once in the file, in the
top-level key allowlist. Parsing uses plain `yaml.Unmarshal` with no
`KnownFields(true)`, and `checkUnknownKeys` inspects only top-level keys, so a
missing `name:` silently yields `""`. `rela validate` delegates to the same
`validate()` and passes clean.

So a schema with a bare `- on: {...}` / `do: [...]` loads, validates, and runs
with `Name == ""`. Either soften the doc sentence to describe convention rather
than guarantee, or add `validateAutomations` to make it true.
