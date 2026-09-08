---
id: AM-acl-affordance-grants-reject-unknown-keys
type: automated-measure
title: Affordance grant structs reject unknown keys, and the guard is mutation-tested
description: 'A table test asserting that FieldGrant, OptionGrant and RelationGrant fail the policy load on an unrecognized key — across fields:, visible:, options:, relations: and the nested RelationGrant.Fields — plus a round-trip test pinning that supported keys still load and RelationGrant''s *bool tri-state survives. The guard was mutation-verified: short-circuiting rejectUnknownGrantKeys fails the table test.'
kind: test
location: internal/acl (TestLoadPolicy_AffordanceGrantUnknownKey_Rejected, TestLoadPolicy_AffordanceGrantSupportedKeys)
status: proposed
---

Extends [[AM-acl-config-structs-reject-unknown-keys]] from one struct to the
three affordance-grant structs, which is where the same defect was still live.

The `when:` key is the reason this matters more than a typo guard usually
would. Every other key in these structs fails closed — a dropped
`field:`/`option:`/`relation:` becomes `""`, matches no declared property, and
`internal/affordances` rejects it by name. `when:` has no such backstop, and
its absence means *grant unconditionally*, so a misspelling widens access on
the `visible:` (read redaction) block.

Two assertions plus a mutation check, and the third is the one worth copying.
The rejection test alone would pass against a parser that refuses everything;
the round-trip test stops over-tightening. But neither proves the guard is
*reached* — a test can pass because the code under it is unreachable. Disabling
`rejectUnknownGrantKeys` and confirming the table test goes red is what
establishes that. BUG-NRCJ9E shipped a fix whose test iterated the very
registry it was meant to guard, and only code review caught it; the mutation
step is the cheap defence against repeating that.

The process lesson, which belongs with the measure rather than in either bug:
when a fix establishes a rule about a class of defect, the same change must
enumerate the class. Here the enumeration is two greps — structs with yaml
tags, minus structs with an `UnmarshalYAML` — and the difference is the work
item. The preceding fix stated the rule without running them, so three
instances of the defect it described stayed live.
