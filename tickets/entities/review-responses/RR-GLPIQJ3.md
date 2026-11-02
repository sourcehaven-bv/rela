---
id: RR-GLPIQJ3
type: review-response
title: Services.Close stopped the hint sender before the writers
finding: closeChannelHints ran first in Services.Close. The job queue drain and the realtime worker stop ran after it, so channel writes made during the drain were dropped.
severity: significant
resolution: 'retireChannelHints runs last in stopBackgroundServices: after mail, sweeps, the realtime worker and the job queue have stopped, it flushes and then detaches. Local.Stop at the end of Close stops the sender.'
status: addressed
---
