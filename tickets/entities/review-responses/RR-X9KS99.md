---
id: RR-X9KS99
type: review-response
title: Entity export now applies the request world; comment stale and untested
finding: export.go hands worldFromContext(ctx).visibility() to the resolver, so a faced world would authorize one face while the export_render override renders by bare id. The comment still says the reader reads by bare id.
severity: significant
resolution: 'Verified _export is not a world-capable path: refuseWorldIncapablePath 422s any named world before the handler, so the resolver only ever sees the default world there and no face mismatch can arise. Rewrote the comment to say so, and moved the entity read after the error and found checks.'
status: addressed
---
