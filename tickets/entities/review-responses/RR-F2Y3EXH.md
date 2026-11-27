---
id: RR-F2Y3EXH
type: review-response
title: A cascade write reuses the outer commit list
finding: A nested update with a commit list on ctx queues its follow-up on the outer list.
severity: minor
reason: The owner of the list runs it after its commit, which is the intended order; no follow-up is lost.
status: wont-fix
---
