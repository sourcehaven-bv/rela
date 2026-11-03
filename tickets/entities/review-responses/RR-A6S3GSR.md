---
id: RR-A6S3GSR
type: review-response
title: Empty-user branch of the 401 rule
finding: The helper accepts an empty user with any tool. The reviewer reads that as a wiring fault that should keep the 500.
severity: minor
reason: A header-only resolver without ChainResolvers yields an empty user when the header is absent. The golden corpus wires exactly that. Both cases mean no resolver named a user so 401 is the right answer. The named-user-unknown-tool case keeps its 500 and is tested.
status: wont-fix
---
