---
id: BUG-YAMLDROP
type: bug
title: Custom UnmarshalYAML alias structs silently drop any field added to the outer type
description: Types with a custom UnmarshalYAML (CopyDef, WorldDef, ACLBypass, ScanPolicy) decode through an inner alias struct with an explicit field list. Adding a field to the outer struct is not enough — if it is missing from the alias it parses clean and arrives as the zero value, so a declared setting silently becomes its default with no error anywhere. Hit for real by `after:` on CopyDef.
priority: medium
status: backlog
why1: CopyDef.UnmarshalYAML decodes into a local `raw` struct that lists fields explicitly, and `After` was added to CopyDef but not to `raw`.
why2: The alias-struct pattern exists to avoid infinite recursion into UnmarshalYAML, and duplicating the field list is the standard way to do that — so the duplication is deliberate and looks correct.
why3: YAML decoding ignores unknown keys by default, so a key present in the document with no matching alias field is not an error; it is silently dropped.
why4: The failure is invisible at every stage — the schema loads clean, validation sees the zero value as "not set", and the feature simply behaves as though the operator never wrote the line.
why5: A structural invariant (two field lists must stay in sync) was maintained by convention, with nothing mechanical asserting it — the same class the codebase converts to guard tests elsewhere.
prevention: A reflection guard test comparing the outer type's field count against its alias, failing with "add the new field to raw in UnmarshalYAML". This is the ceilingguard_test.go pattern — the codebase already converts prose invariants into guard tests when the failure mode is silent.
---

## Symptom

`internal/metamodel/types.go`, `CopyDef.UnmarshalYAML`:

```go
type raw struct {
    From      string            `yaml:"from"`
    To        string            `yaml:"to"`
    Fields    any               `yaml:"fields,omitempty"`
    Relations map[string]string `yaml:"relations,omitempty"`
    Guard     CopyGuard         `yaml:"guard,omitempty"`
}
```

Adding `After` to `CopyDef` was not enough. Until it was also added to `raw`,
`after: discard` parsed **clean** and arrived as `""`.

## Impact

`""` is `keep`. So an operator writing

```yaml
copies:
  promote-policy:
    from: policy@draft
    to:   policy@published
    after: discard
```

would get a promote that silently **stops consuming the draft** — no error, no
warning, the schema loads, and the only symptom is drafts accumulating that
should have been consumed. Caught during TKT-C1XUA8 PR-E only because the
load-refusal tests failed; without them it would have shipped.

## Scope

Every type in `internal/metamodel` with a custom `UnmarshalYAML` and an inner
alias has the same hazard. Known: `CopyDef`, `WorldDef`, `ACLBypass`,
`ScanPolicy`. A comment now warns at `CopyDef`'s alias, which stops the
immediate repeat but not the class.

## Fix

A guard test per type (or one table-driven test over them):

```go
require.Equal(t, reflect.TypeOf(CopyDef{}).NumField(), aliasFieldCount,
    "a field was added to CopyDef but not to raw in UnmarshalYAML — it will "+
    "parse clean and arrive as the zero value")
```

Precedent: `internal/acl/ceilingguard_test.go` does exactly this for the
role-resolution ceiling — scans the package, fails on a new unclean file, uses
an exemption list. Same move, different invariant.
