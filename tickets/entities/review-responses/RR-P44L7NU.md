---
id: RR-P44L7NU
type: review-response
title: drain relies on flushed advancing after writes
finding: If runSSELoop ever advanced flushed before writing frames, the negative tests would pass falsely.
severity: minor
reason: The drain doc comment states the invariant and the positive tests in the same file would fail on such a change; the diff stays test-only.
status: wont-fix
---
