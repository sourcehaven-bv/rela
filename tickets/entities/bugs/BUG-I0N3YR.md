---
id: BUG-I0N3YR
type: bug
title: 'A misspelled when: key in an affordance grant is dropped so the grant becomes unconditional'
description: 'FieldGrant, OptionGrant and RelationGrant had no strict UnmarshalYAML, and yaml.v3 drops keys it cannot map. Each carries a `when:` conditioning the grant on a predicate, and an absent When compiles to a nil program which the resolver treats as "grant unconditionally". So `wehn:` silently promotes "editors may see salary IF they are HR" to "editors may see salary" — on the `visible:` block that is a read-side field disclosure, not merely a wider write gate.'
priority: high
effort: s
why1: A grant written with a misspelled `when:` loads clean with When empty, and PolicyResolver.compile maps an empty When to a nil program, which every affordance path reads as an unconditional grant.
why2: FieldGrant/OptionGrant/RelationGrant had no UnmarshalYAML; yaml.v3's default is to ignore unmappable keys rather than error.
why3: 'The sibling keys in the same structs fail CLOSED, which hid this: a dropped `field:`/`option:`/`relation:` becomes "", matches no declared property, and internal/affordances rejects it by name. Only `when:` — the one key whose absence means MORE access — had no downstream backstop.'
why4: The strictness added in BUG-BRZX6O was applied to the struct whose bug had been reported (RoleRelationDef) rather than to every struct sharing its failure mode, so the audit stopped at the reported symptom.
why5: 'Same systemic cause as BUG-BRZX6O, one level up: there is no invariant that config parsing under acl.yaml fails closed, so each struct gets whatever strictness its author happened to consider. BUG-BRZX6O recorded that rule as prose in an automated-measure; prose does not enumerate the remaining offenders, so the identical defect stayed live in three more structs.'
status: done
prevention: 'FieldGrant, OptionGrant and RelationGrant now reject unknown keys via a shared rejectUnknownGrantKeys helper, and the guard is mutation-tested (disabling it fails the new table test, so the test cannot rot into a tautology). The broader lesson, which the BUG-BRZX6O measure did NOT capture: when a fix establishes a rule about a class of defect, the same commit must ENUMERATE the class, not just state the rule. The enumeration here is mechanical — grep the package for structs with yaml tags and diff against those with an UnmarshalYAML — and finding three more offenders took one grep once the question was asked.'
---

## Symptom

A conditional grant with a typo in the condition key:

```yaml
roles:
  editor:
    read: [note]
    visible:
      note:
        - field: salary
          wehn: "has_role(current_user, 'hr')"   # typo
```

`wehn:` is not a field on `FieldGrant`, so yaml.v3 drops it. `When` stays
`""`, and `PolicyResolver.compile` returns a nil program for an empty
`When` — which the resolver treats as *unconditional*.

The operator wrote "editors may see salary if they are HR" and deployed
"editors may see salary". Nothing reports it: the policy is valid, the
field name is real, and the grant works — just for everyone holding the
role.

This is `visible:`, so it is field-level **read** redaction. The
equivalent typo on `fields:` widens writes, on `options:` widens which
enum values may be set, and on `relations:` widens link affordances.

## Why the sibling keys hid it

The other keys in these structs fail *closed*, which is why this survived
review. A dropped `field:`/`option:`/`relation:` becomes `""`, which
matches no declared property, and `internal/affordances` rejects it with a
named error (`validateField` / `validateOption`). So three of the four keys
had a downstream backstop and the fourth — the only one whose absence means
*more* access — had none.

## Relationship to BUG-BRZX6O

Same class, found by applying that bug's own rule to the rest of the
package. BUG-BRZX6O fixed `RoleRelationDef` and recorded the general
principle (a config struct's parser must fail closed; ask which direction a
dropped key fails). It did not enumerate the other structs sharing the
shape, so the rule was written down while three instances of the defect it
described stayed live.

That is the why5 worth keeping: stating a rule is not the same as applying
it. The enumeration is mechanical —

```console
$ grep -n "^type .* struct" internal/acl/policy.go
$ grep -n "func (.*) UnmarshalYAML" internal/acl/policy.go
```

— and the diff between those two lists is the work item.

## Fix

A shared `rejectUnknownGrantKeys` helper, called from a new
`UnmarshalYAML` on each of the three structs. It walks the mapping node's
keys and refuses anything outside the known set:

```console
$ rela list
appbuild: load acl.yaml: acl: parse /srv/rela/acl.yaml: fields/visible grant
at line 7: unknown key "wehn" (want field, when)
```

The error names the **source line** rather than an index. An index here
would number the keys of one grant's mapping and read as though it named
which grant was wrong — the first draft did exactly that and reported
`[1]` for a typo in the first grant.

`RoleDef` itself was left lenient deliberately: a dropped verb key there
(`creat:` for `create:`) removes grants, which fails closed. It is worth
fixing for operator ergonomics but is not this bug.

## Verification

- Table test over all four structs including the nested
  `RelationGrant.Fields` case.
- A round-trip test pinning that supported keys still load — in particular
  that `RelationGrant`'s `*bool` fields still distinguish "unset" from an
  explicit `false`, since the new unmarshaller decodes via a type alias.
- Mutation-tested: short-circuiting the helper to `return nil` fails the
  table test, so the guard is load-bearing rather than decorative.
- End-to-end against the built binary for both a `visible:` and a
  `relations:` typo.
