---
id: RR-9CDOFLM
type: review-response
title: Elevation keys accepted on an llm action
finding: allow_acl_bypass and elevated_updates were accepted on an llm action, implying elevation that does not apply.
severity: minor
resolution: Config load refuses them on an llm action; tested.
status: addressed
---
