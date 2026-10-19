---
id: RR-7DGRRE
type: review-response
title: Entity reads bypass the AllowAllReader capability the plan and CLAUDE.md prescribe
finding: 'internal/appbuild/realtime_triggers.go:19-85 and realtime_worker.go:268-271. entitySource queries the raw store.Store. Plan sections 0 and 5 and the CLAUDE.md read-out rule say a system job gets an explicit visibility.AllowAllReader capability at wiring, keeping its system:* principal for audit, so allow-all is visible at the wiring site. The worker ctx also carries no principal.Principal{system:realtime}. Fix: build the source over the AllowAllReader at wiring and stamp the worker ctx with the realtime principal. Otherwise, record in the godoc why a raw read is correct here, as anchorExistence does, and update the plan.'
severity: minor
resolution: entitySource now reads through visibility.NewAllowAllReader, handed out at the wiring site, and the leader runs on a ctx stamped with the system:realtime principal.
status: addressed
---
