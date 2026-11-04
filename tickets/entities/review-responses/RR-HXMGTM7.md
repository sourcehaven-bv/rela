---
id: RR-HXMGTM7
type: review-response
title: Sort is inconsistent with invalid values mixed in
finding: Unparseable values fall back to string order and can form a cycle.
severity: minor
reason: Deferred to a follow-up decimal ticket; compareIntegers has the same pattern today; fix both together by ordering invalid values last.
status: deferred
---
