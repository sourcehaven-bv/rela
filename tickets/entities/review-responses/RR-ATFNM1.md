---
id: RR-ATFNM1
type: review-response
title: 'Sync redacted fetch: markdown Content is not field-redacted (same leak class as RR-FOD7IB, relocated to the body)'
finding: 'The planned redacted sync fetch pipes the entity through serializer.forWire -> stripHiddenProperties, which redacts result.Properties and rewrites _title but NEVER touches Content (verified: affordances.go:913-934 only mutates result.Properties). The sync body (syncEntityBody/syncRelationBody) carries Content. So the redacted fetch strips hidden properties but ships the full markdown body verbatim. If any deployment''s visible: model assumes the body is access-controlled, or a hidden property''s value is duplicated into the body (trivial with markdown+frontmatter storage/templates), the sync GET leaks it — the same class of bug as RR-FOD7IB, relocated from Properties to Content. Note: this is ALSO a pre-existing gap on the web read path (it too ships the full body), so the sync change does not introduce it — but ''route sync through the redacted read path'' does NOT fully close the leak, and the plan must say so rather than implying content is covered.'
severity: significant
resolution: 'Ruled OUT OF SCOPE (user, 2026-08-08). visible: is a property-values-only guard — rela has NO body-level guards anywhere today (neither the web read path nor sync redacts markdown Content; stripHiddenProperties operates on Properties keys only). So shipping the full body is the existing contract on every read path, not a regression this ticket introduces. This ticket is scoped to entity properties + relation meta, matching what the web read path already enforces. Body-level redaction, if ever wanted, is a separate new mechanism spanning both read paths = its own ticket. Acceptance criteria + design will state markdown-body redaction is explicitly out of scope, consistent with CLAUDE.md ''values only''.'
reason: 'No body-level guards exist in rela; visible: is property-values-only. Shipping the full body is the current contract on all read paths, not introduced here. Body redaction would be a new cross-path mechanism = separate ticket.'
status: wont-fix
---

## Finding (design-review C1)

The planned redacted sync fetch pipes the entity through `serializer.forWire` →
`stripHiddenProperties`, which redacts `result.Properties` and rewrites `_title`
but **never touches `Content`** (verified: `affordances.go:913-934` only mutates
`result.Properties`). The sync body (`syncEntityBody`/`syncRelationBody`)
carries `Content`. So the redacted fetch strips hidden *properties* but ships
the full markdown *body* verbatim.

If any deployment's `visible:` model assumes the body is access-controlled, or a
hidden property's value is duplicated into the body (trivial with
markdown+frontmatter storage / templates), the sync GET leaks it — the exact
class of bug as RR-FOD7IB, relocated from `Properties` to `Content`.

Note: this is ALSO a pre-existing gap on the web read path (it too ships the
full body), so the sync change does not *introduce* it — but it means "route
sync through the redacted read path" does NOT fully close the leak, and the plan
must say so explicitly rather than implying content is covered.

## Decision needed

Is `visible:` a **property-values-only** guard (per CLAUDE.md: "hides property
**values only**")? If yes → document that the markdown body is out of scope for
`visible:` redaction on BOTH the web and sync read paths, and the ticket's
acceptance criterion is scoped to properties + relation meta. If the body must
be covered, that is a larger, separate concern (the web path lacks body
redaction too) and should be its own ticket — not silently assumed done here.

## Recommended resolution

Scope this ticket to property + relation-meta redaction (matching what the web
read path already does), and state in the design + acceptance criteria that
markdown-body redaction is explicitly out of scope / deferred, consistent with
the CLAUDE.md "values only" framing. Confirm no in-tree deployment relies on
body redaction.
