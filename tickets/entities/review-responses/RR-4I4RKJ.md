---
id: RR-4I4RKJ
type: review-response
title: No empty run id guard in subscription job
finding: runTaskJob guards runID == ""; runSubscriptionJob does not.
severity: nit
reason: That guard handles jobs enqueued by older releases; subscription jobs are new and always carry a run id. beginRun still refuses an unknown run.
status: wont-fix
---
