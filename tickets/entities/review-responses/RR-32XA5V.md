---
id: RR-32XA5V
type: review-response
title: Field gate runs before ACL authorize — creates an existence + field-policy oracle
finding: 'The plan places FieldGate.CheckFieldWrite after computing the set/unset diff but BEFORE authorizeAndAudit. This inverts the project''s settled ordering. dataentry does the opposite deliberately: gateRead is the FIRST thing in the update handler (write_handler.go:322) with the comment ''runs BEFORE getEntity (RR-NGMI timing) AND before body parse / If-Match / IsLocked so the only observable for "this id exists but you can''t see it" is the same 404 as "this id doesn''t exist" (RR-FGUZ). A 400 / 412 / 422 here would be an existence oracle.'' validateFieldWrite only runs at write_handler.go:376, after the gate, the read, IsLocked and If-Match. Under the plan''s order an unauthorized caller gets three distinguishable outcomes: ErrEntityNotFound (absent), AffordanceDenialError (exists, field denied), or ACL forbidden (exists, field allowed). Worse, AffordanceDenialError carries Rule/Path/Reason/Attribution (affordances.go:339-361) so it names the denying rule/role, and FieldVerdicts are value-dependent (verdicts differ by the entity''s own property values), so the allow-vs-deny difference leaks entity CONTENT to a principal with no write authority at all. Correct order: authorize first, field-gate second.'
severity: critical
resolution: |-
    ACCEPTED, with the rationale NARROWED. Order corrected to: raw GetEntity -> IsLocked guard -> authorizeAndAudit -> FieldGate -> apply diff -> updateCore.

    Rationale narrowed per the project's config-is-public posture (root CLAUDE.md: 'The configuration is not a secret; the data is'). The original finding cited AffordanceDenialError.Attribution naming the denying rule/role as a leak — that part is WITHDRAWN. acl.yaml is operator-authored, lives in a routinely-public repo, and CLAUDE.md explicitly says a 403 naming the missing permission is the right answer for a config-declared capability. Naming the rule is a feature, not a leak.

    What SURVIVES, and is why the reorder is still required: (1) EXISTENCE — not-found vs denied distinguishes whether an entity id exists, and entity existence is explicitly secret (row-level rule, DEC-ZBI39P: 'a hidden entity is nonexistent'). (2) CONTENT — field verdicts are value-dependent (they evaluate predicates against the entity's own property values), so an allow-vs-deny difference on a fixed patch reveals stored property values to a caller with no write authority. Both are in the 'data is secret' half of the posture, not the config half.

    So the defect is real and the fix is unchanged; only the justification is trimmed to the two claims that hold.
status: addressed
---

## Evidence

`internal/dataentry/write_handler.go:318-324`:

```go
// before body parse / If-Match / IsLocked so the only observable
// for "this id exists but you can't see it" is the same 404 as
// "this id doesn't exist" (RR-FGUZ). A 400 / 412 / 422 here would
// be an existence oracle.
if !h.gateRead(w, r, typeName, entityID) {
    return
}
```

`internal/dataentry/write_handler.go:370-379` — the field gate runs much later:

```go
// Affordance parity (TKT-G7N5): reject writes that conflict with
// what the resolver would have surfaced on GET.
if denial := h.affordances.validateFieldWrite(
    r.Context(), entity, req.Properties, req.PropertiesUnset,
); denial != nil {
```

The delete handler repeats the pattern (`write_handler.go:488-491`) noting
RR-3532: gateRead before AuthorizeWrite *"so a hidden target 404s, not
403-with-rule_id"*.

## Resolution

Reorder. See RR (finding 2) for the constraint that `PatchEntity` must read
before authorizing in order to learn the entity type for the ACL subject — the
resulting order is:

```
GetEntity (raw) → IsLocked guard → authorizeAndAudit → FieldGate → apply diff → updateCore
```

The set/unset diff does not depend on the ACL decision, so nothing forced the
plan's original order.
