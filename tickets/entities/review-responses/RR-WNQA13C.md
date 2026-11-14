---
id: RR-WNQA13C
type: review-response
title: Require permission on llm actions
finding: 'Every call sends data to an external provider and costs money. An llm action without permission: is callable by anyone who can read the subject. Require permission: at load time.'
severity: significant
resolution: Config load rejects an llm action without permission:.
status: addressed
---
