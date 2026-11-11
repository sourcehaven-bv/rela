---
id: RR-M82A6QZ
type: review-response
title: Download reads the message twice
finding: OpenFile calls Get and then the store's OpenFile, which reads the message again.
severity: minor
reason: The service needs the message type to check that the property is still declared as a file, and the store must check the id itself. Downloads are rate limited, so the extra read is bounded.
status: wont-fix
---
