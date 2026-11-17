---
id: RR-441BKLI
type: review-response
title: 'Design: allowlist classification test must cover all of flows.yaml'
finding: 'DecodeTypes covers only triggers:. Assert no editable pattern reaches flows: so removing a flow stays refused.'
severity: minor
resolution: TestAllowlist_ClassifiesEveryKey covers flows.yaml through triggerPaths built from triggers.DecodeTypes. TestAllowlist_FlowsSectionLocked checks flows.* is locked.
status: addressed
---
