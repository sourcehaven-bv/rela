---
id: RR-OPQU0M
type: review-response
title: No behavioural coverage for keep_on_add_another on relations
finding: keep_on_add_another was typed in Go and TS and documented as the primary use case ('a project, an assignee'), but no test exercised it on a relation. The unit suite covered only a property; the e2e fixture task_add_another had no relations block at all. That absence is why the pendingCardChanges leak shipped.
severity: significant
resolution: 'Added a kept `implements` relation to the e2e fixture form plus an e2e test that creates two records and verifies via the API that the SECOND entity really carries the kept relation. Added two unit tests: one for a kept outgoing relation surviving and remaining sendable, one asserting an unkept incoming relation does not reach record 2. The picker stub now emits the real events instead of only counting mounts.'
status: addressed
---
