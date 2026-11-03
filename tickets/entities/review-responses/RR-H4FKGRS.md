---
id: RR-H4FKGRS
type: review-response
title: Negative assertions rely on sleeps
finding: Some tests assert absence after a short sleep (hints_test resync limit, notify_test after cancel).
severity: nit
reason: The feed test now waits on the start gap. The remaining two check that nothing more happens; there is no event to wait on for that. A false pass only weakens the check and cannot fail spuriously.
status: wont-fix
---
