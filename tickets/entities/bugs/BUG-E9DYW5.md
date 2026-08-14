---
id: BUG-E9DYW5
type: bug
title: 'ICS feed serves `visible:`-redacted properties verbatim'
description: A property hidden from a principal by a `visible:` ACL grant was served verbatim in the ICS feed, while every other HTTP read shape omits it. Row-level gating was unaffected; only field-level redaction was missing. Found during the CalDAV security review (PR #1308), which fixed the same gap on its own read path.
priority: high
why1: The ICS feed's mapEntity reads properties straight off the raw entity (e.GetString), so a property a role's `visible:` grant hides is rendered into the event.
why2: Field-level redaction is applied by the data-entry serializer, and the feed does not go through it — it builds a calfeed.Event directly from the entity.
why3: The feed was built as a projection of entities to a calendar model, and row-level gating (feedEntitySource.listType) was the visible ACL concern; field-level redaction is a second, orthogonal cut that was not considered at the seam.
why4: Redaction is enforced per-read-shape rather than structurally, so every NEW read shape has to remember it. docs/acl-security.md asserts it applies to 'every HTTP read shape', which reads as a guarantee rather than a convention to re-implement.
why5: There is no type-level barrier between 'raw store entity' and 'entity safe to render outward'. Both are *entity.Entity, so a new read path compiles and works while silently skipping redaction.
prevention: Redaction now runs at the single mapping chokepoint in each feed path, pinned by tests that fail if the call is removed (including an ordering test for filter-before-redact). The durable fix is a distinct type for 'redacted, safe to render' so a raw entity cannot reach a serializer by accident — tracked separately.
status: done
---

## Symptom

A property hidden from a principal by a `visible:` ACL grant is served verbatim
in the ICS feed (`/api/v1/_feeds/<name>.ics`), while every other HTTP read shape
omits it.

A feed source mapping `description: notes` renders `notes` into the event
DESCRIPTION for any principal who passes the **row** gate, regardless of whether
their role may read that field.

## Scope

Row-level gating was never affected: `feedEntitySource.listType` resolves the
`ReadQuery` verdict fail-closed, so an entity the principal cannot read is
absent from the feed. Only **field-level** redaction was missing.

Found during a security review of the CalDAV work (PR #1308), which inherited
the same gap from this code and fixed it on its own read path. This ticket is
the pre-existing feed half.

## Fix

`mapEntity` now applies the `visible:` redactor before reading any property into
the event.

**Ordering is load-bearing.** Redaction runs AFTER the `where:` filter, never
before. Redacting first would make a hidden property read as empty inside the
filter, so feed *membership* would vary per principal — the same feed would
contain different events for different readers, and an entity would silently
drop off the calendar because a field the reader cannot see was filtered on.
Which entities a feed selects is an operator-authored decision; what their
fields say is the reader's business.

One accepted consequence: if a feed is anchored on a date property the reader
may not see, entities have no usable date for them and leave that reader's
calendar entirely. Deliberate — an event whose date you may not read is not one
you should be shown.

## Verification

Three tests, each verified to FAIL against the fix being removed:

- a hidden property does not reach the rendered event
- hiding the **filtered** property does not change feed membership (pins the
ordering: moving redaction before the filter drops the event)
- redaction copies rather than mutating the shared store entity, which would
redact it for every other reader in the process — including write-prep reads,
where a missing property is an erasure
