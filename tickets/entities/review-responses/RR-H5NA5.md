---
id: RR-H5NA5
type: review-response
title: User-configurable date format setting (out of scope)
finding: Locale-dependent output means collaborators on the same database see different strings. A schemaStore.config.dateFormat would let teams pin a project-wide format. Out of scope here, but the centralization is the prerequisite.
severity: nit
reason: Out of scope for this ticket per the reviewer's own framing. The centralization (exported formatDate, DATE_FORMAT_OPTIONS) is the prerequisite and is now in place. Worth a separate feature ticket if a user actually requests it.
status: deferred
---
