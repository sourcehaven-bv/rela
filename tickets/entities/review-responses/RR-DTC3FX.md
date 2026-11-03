---
id: RR-DTC3FX
type: review-response
title: channelsSection duplicates busyFields and is misnamed
finding: 'internal/projectsetup/validate.go:295-311 walks kanban card fields to detect a busy field, which repeats dataentryconfig.busyFields (channels.go:224-237). It now returns more than the channels section. Fix: export one `dataentryconfig.HasRealtimePresentation(*Config) bool`, use it in both places, and rename channelsSection to realtimePresentation.'
severity: nit
resolution: Exported dataentryconfig.HasRealtimePresentation and used it in projectsetup (renamed realtimePresentation) and in ValidateChannels, so the walk lives in one place.
status: addressed
---
