---
id: RR-N1FDN2
type: review-response
title: 'Kanban busy: key fits the conventions but is narrow'
finding: 'A standalone card field naming a channel type matches comments: true (internal/dataentryconfig/config.go:1503) and the type-keyed channels: block, so busy: run is consistent. It only ever shows working states. If a later ticket wants input_required on cards too, a session: run field with a display mode would avoid a second key. The plan should record this choice in Alternatives.'
severity: nit
resolution: 'Recorded in Alternatives: a session: field with a display mode was considered; busy: is kept for consistency with comments: and the channels: block, and could later become a short form.'
status: addressed
---
