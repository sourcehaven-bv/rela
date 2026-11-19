---
id: TKT-W4F7VCB
type: ticket
title: 'Flow guard may read a field every visible: block shows'
kind: enhancement
priority: medium
effort: s
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

## Description

A flow guard or template may not read a property that acl.yaml restricts,
because the run's state (and each branch's verdict in the run view) is shown to
every reader of the subject. `acl.Policy.FieldRestricted` treated every property
of a type as restricted as soon as any role declared a `visible:` block for it,
even a property that every block lists without a `when:`.

So harbourmaster cannot add a customer `visible:` block on `account`: the signup
guard reads `entity.terms_accepted` and the flow config is refused. Customers
can then read internal account fields (org_id, eb_relation_id, vat_valid,
reverse_charge, terms_accepted_at).

## Fix

Narrow `FieldRestricted`: a role's `visible:` block keyed by the type restricts
a property only when it leaves the property out or lists it only under a
`when:`. A block keyed by a face or by `*` still restricts every property.
Client baseline rules are unchanged. A role without a block shows every property
and the role union only adds, so a property every block lists unconditionally is
visible to every reader of a row, and a guard verdict over it discloses nothing.

## Acceptance

- A guard over a property every `visible:` block lists without `when:` loads.
- A guard over a property a block leaves out, or shows only under `when:`,
is still refused.
- GUIDE-flows documents the rule.
