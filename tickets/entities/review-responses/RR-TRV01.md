---
id: RR-TRV01
type: review-response
title: 'Field gate missed unconditionally-hidden fields (visible: closed world)'
severity: significant
status: addressed
finding: |-
    [security] ConditionallyVisible refused a property only when a `visible:`
    grant carried a `when:`. But `visible:` is a CLOSED WORLD per role —
    applyFieldGrants (internal/affordances/resolver.go) opts the whole
    dimension in as soon as a role declares it, so every field the role does
    not list is hidden, with no `when:` string anywhere to detect.

    An operator writing `visible: [name, title]` to hide `salary` produced
    ConditionallyVisible(person, salary) == false, so the gate composed it
    into the endpoint predicate. A principal who may READ person rows could
    then binary-search the hidden salary — the exact 'redacted field VALUE'
    leak the gate exists to prevent. This is the likelier spelling of the
    two, and the one the when:-only check missed.
resolution: |-
    ConditionallyVisible now returns true in BOTH cases: a conditional grant,
    and a role that declares a visible: block for the type without listing
    the property. Verified with a fixture where salary is unlisted; the
    unconditionally-granted field stays filterable.
---
