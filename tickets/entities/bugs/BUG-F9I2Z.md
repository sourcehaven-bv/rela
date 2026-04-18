---
id: BUG-F9I2Z
type: bug
title: generateDataEntryConfig produces invalid YAML when titles contain quotes
description: generateDataEntryConfig in cmd/rela-desktop/main.go builds the first-run data-entry.yaml by string-concatenating YAML fragments. Titles derived from entity/property names are embedded inside double-quoted scalars with no escaping, so names containing a double-quote, backslash, or YAML-special char produce invalid or silently-rewritten YAML. Desktop first-run scaffolding is the user's entry point, so a corrupt file here is disproportionately painful.
priority: low
effort: s
why1: generateDataEntryConfig built YAML by string-concatenating fragments with %q/"%s" embedding
why2: Hand-built YAML was expedient when the function only handled simple identifiers
why3: No roundtrip test existed to prove the output was valid for all possible inputs
why4: Test-writing culture favored substring assertions over structural/parseability checks
why5: The scaffold generator was treated as a one-off, not a production YAML emitter with user-derived strings
prevention: Always use yaml.Marshal or a documented tested encoder for YAML/JSON emission; reject PRs that build structured formats with fmt.Fprintf. New automated-measure 'generate-dataentry-yaml-roundtrip-test' enforces this for the scaffold specifically.
status: done
---

## Description

`cmd/rela-desktop/main.go:generateDataEntryConfig` builds a `data-entry.yaml`
scaffold by string-concatenating YAML fragments with `fmt.Fprintf(&sb, " title:
\"%s\"\n", titleCase(propName))`. The title value is embedded inside double
quotes but only runs through `titleCase()` — no YAML escaping.

If a metamodel defines an entity-type or property name that contains a
double-quote, backslash, or any unicode character that YAML treats as special in
a double-quoted scalar (e.g. `\n`, `\t`, `\"`), the generated YAML either
becomes invalid or silently rewrites the value.

## Impact

- Low: rare in practice — typical entity/property names are `snake_case` or `kebab-case` identifiers without quotes.
- But: the desktop app's first-run config scaffolding is the user's introduction to the product. A corrupt `data-entry.yaml` at this point leaves the user stuck with no good error message.
- Related: this is a latent class of bug — any future metamodel feature that allows e.g. `description` values in the scaffold would surface it.

## Reproduction

1. Create a metamodel with an entity type named (hypothetically) `foo"bar`.
2. Run the desktop app's "new project" flow.
3. Observe the generated `data-entry.yaml` — the title field breaks parsing.

## Fix

Replace the hand-built YAML with `yaml.Marshal` of a typed or `map[string]any`
representation. The project already depends on `gopkg.in/yaml.v3`. The helpers
`titleCase`, the entity-type sort, and the column-limit constants stay; only the
Fprintf/string-concat block is replaced.

**Estimated effort:** s (small). ~80 lines swapped for ~40 lines of struct
construction + one `yaml.Marshal` call.

## Origin

Flagged by the cranky-code-reviewer agent during TKT-AWX7V review as RR-V7SIR
(severity: minor, status: deferred). Deferred from that ticket because the scope
was a tooling/version chore, not a refactor of `generateDataEntryConfig`. Filing
as its own bug here.
