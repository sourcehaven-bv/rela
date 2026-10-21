---
id: RR-CZ2DQC
type: review-response
title: dropUndeclared tells bare-id observers about faced families only through the implicit face
finding: 'internal/store/fsstore/schemas.go:98 copies the watcher''s single-file rule (notify bare-id observers only for the implicit face). But SetSchemas removes whole families, as deleteEntity and softdelete do, and those call notifyLastFaceDelete once per id. A bare-id observer would keep every entity of a removed faced type. Both production observers (bleve, linear) are face-aware, so nothing breaks today. Fix: collect the ids removed and call notifyLastFaceDelete once per id after the loop.'
severity: nit
reason: Both production search observers handle faces, so nothing breaks today; a one-line comment records the behavior.
status: wont-fix
---
