---
id: RR-YMRGY2
type: review-response
title: assemble leaks a started job queue on later error paths
finding: buildRuntimeServices STARTS the job queue, but a later failure in assemble (e.g. buildEntityManager) returned without closing it. Boot-only this was harmless since a failed boot exits the process; making assembly a reload path turns it into one leaked queue per bad schema save — on postgres, a connection pool each time.
severity: critical
resolution: Named the return values and added a deferred closeJobQueue on the error path; factored closeJobQueue so the teardown path and the error path share one bounded close.
status: addressed
---
