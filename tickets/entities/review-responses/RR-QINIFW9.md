---
id: RR-QINIFW9
type: review-response
title: Missing coverage for some conflict gate cases
finding: 'No tests for relation-side visible: redaction, a property hidden on one side only, a faced file with a face-only grant, or a gate error.'
severity: minor
resolution: Not fixed in this change; see reason.
reason: 'The side naming a different hidden id and the non-conflicted hidden file are now covered. The rest need fixtures (relation visible: resolver, faced types on the conflict app) beyond this fix; the gate paths they exercise are shared with tested read routes.'
status: deferred
---
