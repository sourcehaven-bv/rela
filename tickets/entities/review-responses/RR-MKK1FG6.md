---
id: RR-MKK1FG6
type: review-response
title: 'Security: write check ignores subject type and wildcard or abstract grants'
finding: set and relate write to on.entity which is not judged; wildcard and abstract grants in acl.yaml name no concrete type so create.type passes.
severity: significant
resolution: 'appbuild.CheckTriggerACL judges each added or changed trigger against the parsed acl.yaml (checked against the draft schema) and the editor''s request; configedit calls it through the TriggerChecker interface before any early return. Writes are judged per expanded on.entity type and face; the policy is parsed fresh and expanded over the draft schema, so wildcard and abstract grants apply; grant keys match type, type@face and "*". The editor must hold the verb in both the saved policy and the live request (client ceiling included) and the field on every face when the role declares fields:. Tests: TestCheckTriggerACL.'
reason: Judging the subject type by name would refuse nearly every trigger in a project whose acl.yaml lists its types, and wildcard or abstract grants need the parsed policy. Same posture as the old automations editor. Tracked in TKT-CB1YV7D; documented that config:edit is administrator access.
status: addressed
---
