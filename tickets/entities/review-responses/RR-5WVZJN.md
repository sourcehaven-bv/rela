---
id: RR-5WVZJN
type: review-response
title: triggered_by format differs between sections 0.7 and 2
finding: '0.7 uses channel:<key>#<seq>/<action> and section 2 uses channel:<anchor>/<type>[/<name>]#<seq>/<action>, and neither includes the face for anchor: face channels. Pin one format in PromoteRequest.Origin''s godoc and assert it in TestPromote_TriggeredByInAudit.'
severity: nit
resolution: 'One format pinned in PromoteRequest.Origin godoc and asserted in TestPromote_TriggeredByInAudit: channel:<anchor>[@face]/<type>[/<name>]#<seq>/<action>.'
status: addressed
---
