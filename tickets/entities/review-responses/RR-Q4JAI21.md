---
id: RR-Q4JAI21
type: review-response
title: Without acl.yaml nobody holds a role
finding: The role checks find no roles without a policy, so no step could be completed.
severity: significant
resolution: The flows loader refuses roles in flows.yaml without acl.yaml, so a waiting state always has a policy to check. The nil-policy path in FlowControl treats every role as held for completeness, but no loaded flow reaches it.
status: addressed
---
