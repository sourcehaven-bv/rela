---
id: AM-acl-config-structs-reject-unknown-keys
type: automated-measure
title: Config structs under acl.yaml reject unknown keys at load
description: 'Table tests asserting that a nested acl.yaml config struct fails the policy load on an unrecognized key rather than dropping it. Covers RoleRelationDef (the four write verbs get a message naming relation_grants:, anything else "unknown key") alongside the pre-existing RelationWriteGrant coverage. yaml.v3 ignores unmappable keys by default, so without a strict UnmarshalYAML a typo in a security-relevant block loads clean and enforces nothing.'
kind: test
location: internal/acl (TestLoadPolicy_RoleRelationsVerbKey_Rejected, TestLoadPolicy_RoleRelationsSupportedKeys)
status: proposed
---

Guards the why5 of the verb-key fail-open: the project had no stated invariant
that security-relevant config fails closed on unrecognized input. Strictness was
applied per-struct as each was written, so `RelationWriteGrant` — the block that
*widens* access — got the strict parser, while `RoleRelationDef` — the block
that *restricts* it — got the lenient one. That is precisely backwards.

The direction of the failure is what matters, and it is why a lenient parser is
not a neutral default here. A dropped key in `relation_grants:` removes an
alternative satisfier: the write falls back to the source-type verb grant and is
*more* likely to be denied. A dropped key in `role_relations:` removes the
delegate-X gate entirely, and every consumer reads the resulting empty
`RequiresPermission` as "this relation was never gated" — so the failure is
invisible to `aclaudit` A2 and to the boot warning as well.

Two assertions, both needed. The rejection test alone would pass against a
parser that refuses everything; the round-trip test
(`TestLoadPolicy_RoleRelationsSupportedKeys`) pins that `confers` and
`requires_permission` still load, so the guard cannot be satisfied by
over-tightening.

The rule this generalizes to, for the next struct added under `acl.yaml`: a
strict unmarshaller is the default, and the question to ask before accepting a
lenient one is which direction a dropped key fails. Leniency is only acceptable
where it fails closed.
