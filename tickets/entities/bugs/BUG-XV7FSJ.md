---
id: BUG-XV7FSJ
type: bug
title: A date property is rewritten to a full RFC3339 timestamp on any write
description: 'A hand-authored date-typed property is silently reformatted to a full RFC3339 timestamp the first time anything writes the entity (due: 2026-08-12 becomes 2026-08-12T00:00:00Z). Pre-existing and reproducible on the plain REST path, so not CalDAV-specific; found while building the CalDAV demo, where yaml.v3 decodes an unquoted date to time.Time and GetString then returns empty.'
priority: low
status: backlog
---

## Symptom

A hand-authored `date`-typed property is silently reformatted the first time
anything writes the entity:

```yaml
due: 2026-08-12          # before
due: 2026-08-12T00:00:00Z  # after any write
```

## Not CalDAV — reproduced on the plain REST path

Found while demoing CalDAV, but **confirmed pre-existing**. A plain `PATCH
/api/v1/tasks/{id}` that touches only `title` rewrites `due` the same way:

```
$ curl -X PATCH -H "Origin: ..." -d '{"properties":{"title":"renamed"}}' .../TSK-probe2
PATCH -> 200
due: 2026-08-25T00:00:00Z     # was 2026-08-25
```

Note a `rela update` CLI write does NOT reproduce it, so the trigger is the
API/serialization path rather than the store in general.

## Cause

`yaml.v3` decodes an unquoted date scalar (`due: 2026-08-12`) straight to
`time.Time` rather than a string. The value round-trips through a write as a
`time.Time` and is re-serialized in full RFC3339 form. `entity.Patch` preserves
it faithfully — verified — so the reformatting happens at serialization.

## Why it is low priority

Cosmetic and idempotent: the value still parses, means the same day, and does
not change again on subsequent writes. It is annoying in a git diff (an
unrelated property edit shows a `due` change) and it makes hand-authored files
drift from their authored form.

## Related hazard, worth fixing together

`entity.Entity.GetString` returns `""` for a `time.Time`-valued property, so any
consumer reading a date through it **silently drops the value**. This already
bit the CalDAV mapper (every hand-authored due date vanished from the served
VTODO until `caldav_mapping.go`'s `entityTimeValue` was added), and the same
latent hazard exists in the ICS feed's `mapEntity`
(`internal/dataentry/feed_provider.go:206`), which reads `s.Date` via
`GetString`. A feed over a hand-authored date property would drop those events.

A shared accessor on `entity` — one that accepts both shapes — would close both
this and the serialization asymmetry in one place.
