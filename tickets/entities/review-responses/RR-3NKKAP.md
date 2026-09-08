---
id: RR-3NKKAP
type: review-response
title: Plan's retry rationale cites RetryBounded, but scheduler child jobs that return nil vs error differ in a way the plan should state precisely
finding: 'The plan justifies returning nil (not an error) on suppression by claiming an error would ''trigger RetryBounded retries, which would re-render on every attempt and never succeed.'' The conclusion is right and matches the established convention on this path (skipBadAddress and the ErrNotFound recipient branch both return nil), but the cited mechanism is loose: internal/scheduler/foreach.go:87 sets jobs.RetryBounded on the CHILD job, while internal/scheduler/jobs.go:244,279 use jobs.RetryNever for the scheduler''s own task submissions because ''the scheduler owns retrying''. The plan should cite foreach.go:87 specifically rather than implying a single uniform retry policy, so a reader does not conclude the whole scheduler is RetryBounded.'
severity: minor
resolution: Plan's Technical Approach now cites internal/scheduler/foreach.go:87 as the specific source of jobs.RetryBounded for template child jobs, and notes that the scheduler's own task submissions use jobs.RetryNever per internal/scheduler/jobs.go:203 ('the scheduler owns retrying'). No behaviour change; the return-nil decision was already correct.
status: addressed
---

## Finding

The plan's **Technical Approach** justifies returning `nil` on suppression:

> An error would mark the job failed and trigger `RetryBounded` retries, which
> would re-render on every attempt and never succeed.

The **conclusion is correct** and the convention it appeals to is real:
`skipBadAddress` (`internal/appbuild/scheduled_mail.go:77`) and the
`store.ErrNotFound` recipient branch both return `nil` for "nothing to do", so
suppression returning `nil` is consistent.

But the cited mechanism is imprecise. The retry policy is not uniform across the
scheduler:

- `internal/scheduler/foreach.go:87` — child jobs are `jobs.RetryBounded`.
- `internal/scheduler/jobs.go:244,279` — the scheduler's own task submissions
are `jobs.RetryNever`, with the comment at `jobs.go:203` explaining that "the
scheduler owns retrying: its ladder…".

The plan's phrasing implies one blanket policy. A reader could reasonably
conclude that any error anywhere in the scheduler retries, which is not true and
could mislead a future change on the sibling path.

## Why it matters

Low impact — the decision is right and the code will be right. But the plan is
the artifact a future implementer reads, and an incorrect mechanism attached to
a correct conclusion is the kind of thing that gets copied into a context where
it no longer holds.

## Resolution

Cite `internal/scheduler/foreach.go:87` explicitly as the source of
`RetryBounded` for template child jobs, and note that the scheduler's own
submissions are `RetryNever` per `jobs.go:203`. One sentence; no behaviour
change.
