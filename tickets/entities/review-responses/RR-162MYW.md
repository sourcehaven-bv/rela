---
id: RR-162MYW
type: review-response
title: Permission names in the expectations file are not checked
finding: A typo in permissions becomes a permanent missing line.
severity: minor
reason: 'The typo shows as a ''- permissions: name'' line on the first run and fails CI, so it cannot pass silently. Validating needs the policy, which loads after the file.'
status: deferred
---
