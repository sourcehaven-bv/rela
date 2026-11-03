---
id: RR-QXW7AH
type: review-response
title: Warning codes of the action answer are split across packages and partly undocumented
finding: 'internal/appbuild/realtime_promote.go:22-29 and internal/realtime/promote.go:107 promote_relate_failed and promote_followup_failed are unexported in appbuild, promote_unconfirmed exported in realtime; promote_followup_failed is undocumented. Fix: export all three in realtime and list them in docs/realtime.md and the OpenAPI response description.'
severity: minor
resolution: warning codes WarnPromoteRelateFailed and WarnPromoteFollowUpFailed are exported from realtime next to promote_unconfirmed and used by appbuild; promote_followup_failed is documented in OpenAPI and the realtime guide (453779953 6302a4b22 f2c103e67)
status: addressed
---
