---
id: RR-QF9B7I
type: review-response
title: Worker pool dies permanently when a job's deadline expires while queued
finding: 'Same fatal-worker class as the retry-exhaustion finding, reached by a different route: neoq treats ErrJobExceededDeadline as fatal to the worker goroutine. Job.expired only screened deadlines already past at enqueue, so a job whose deadline lapses WHILE QUEUED killed a worker. This is the designed path, not an edge case — Schedule.NextRun attaches a short deadline to every scheduled job, so a 1m cadence plus one slow job is an outage trigger.'
severity: critical
resolution: neoq is no longer given the Deadline field at all. The effective deadline travels in the payload (__rela_deadline) and dispatch enforces it, returning nil for a lapsed job. Pinned by jobstest PoolSurvivesExpiredDeadlines, which fills every worker, lets queued deadlines lapse, and then requires healthy jobs still run.
status: addressed
---
