---
id: RR-L2UPBS
type: review-response
title: WhoCan checks the verb before the entity
finding: A bad verb with a missing entity now reports the verb error; a bad verb with no candidates used to return an empty result and now errors.
severity: nit
reason: Fail-closed on a bad verb is the intended contract and the CLI enum makes the input unreachable. The verb-before-entity order matches Can and the map report.
status: wont-fix
---
