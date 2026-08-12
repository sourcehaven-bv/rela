---
id: DOCS-CDV003
type: docs-checklist
title: 'Docs: CalDAV go-webdav adapter and protocol surface'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on `caldavBackend` explaining why CalDAV mounts under `/api/` — `attachACLRequest` and `requireVerifiedJWT` are both gated on `isAPIPath`, so a sibling prefix would get no ACL request, no read gate and no JWT gate. Stated as a security requirement, not a URL preference
- [x] `caldav_ctag.go` — why the ctag must be content-derived (store events are droppable and fsstore has no monotonic sequence), why it is spliced per-response keyed on each `<href>` (Reminders sends Depth:1 on the HOME SET), and the cost note that computing it re-renders the collection
- [x] `caldav_write.go` — `staleWriteResponse` (the alias table IS the tombstone), `checkPreconditions` (compare against a freshly rendered ETag, never a cached one), and `refusedWriteResponse` (why a denied write answers 2xx with the ETag suppressed rather than 403/412/409)
- [x] `caldav_proppatch.go` — why a 207 with per-property 403 beats a flat 501
- [x] New `caldavRoutes` type documents why the CalDAV route/middleware helpers hang off a focused type rather than accreting on `App`

## Project Documentation

- [x] `docs/caldav.md` — deployment guide including deletion semantics and the refused-write behaviour
- [x] `docs/caldav-clients.md` — client compatibility, the LOCATION-vs-geofence distinction in Apple Reminders, and observed behaviour on a refused write for both tested clients

## External Documentation

- [x] ~~README~~ (N/A: covered by docs/caldav.md)

**Docs verified:** every non-obvious protocol decision is documented at the
point it is made, with the wire evidence that motivated it.
