---
id: BUG-P3SXOL
type: bug
title: 'Merge queue stalls forever: No Session Trailer is required on develop but never reports on merge_group'
priority: high
status: backlog
---

## Symptom

Every merge-queue entry on `develop` sits in `AWAITING_CHECKS` until the queue's
`check_response_timeout_minutes: 60` expires, then requeues and repeats. Nothing
merges. `origin/develop` stayed pinned at `169de983` while PRs #1536-#1545,
#1526 and #1527 cycled through the queue for hours, each re-running a full green
CI on a fresh `gh-readonly-queue/...` ref.

The CI run itself passes. On queue head `214f56f2` all 22 check runs report
`success` — so the PR page and the queue run both look healthy, which reads
identically to "everything is fine".

## Root cause

The `develop` ruleset requires 20 status checks, including `no-session-trailer /
No Session Trailer`.

`.github/workflows/no-session-trailer.yml` triggers only on `pull_request`:

```yaml
on:
  pull_request:
    types: [opened, synchronize, reopened, edited]
```

Merge-queue entries are tested on an ephemeral `gh-readonly-queue/...` ref,
which fires `merge_group` — not `pull_request`. So the check never reports
there, and a required check that never reports leaves the entry queued until
timeout, forever.

`ci.yml` already carries the `merge_group:` trigger and a comment documenting
exactly this failure mode. `no-session-trailer.yml` was added later and did not
get the same treatment.

## Fix

Add the `merge_group` trigger to `no-session-trailer.yml`, matching `ci.yml`.

On `merge_group` there is no `pull_request` payload, so the reusable workflow's
`BASE_SHA`/`HEAD_SHA` are empty and it falls back to scanning `HEAD~1..HEAD`.
That is the correct range: the merge-queue candidate is a single-parent squashed
commit off the base (verified on `214f56f2`, parent `169de983`), so the fallback
scans exactly the one commit that would land on `develop` — the same commit
whose message would carry a trailer.

## Prevention

The invariant is: **every check listed as required on `develop` must also report
on `merge_group`.** It is currently kept by hand in two places and was already
violated once. Worth a lint that diffs the ruleset's required contexts against
the set of jobs reachable from a `merge_group` trigger, so the next workflow
added as a required check cannot silently wedge the queue.
