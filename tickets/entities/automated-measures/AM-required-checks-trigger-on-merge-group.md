---
id: AM-required-checks-trigger-on-merge-group
type: automated-measure
title: "Every required status check also triggers on merge_group"
kind: ci
location: .github/workflows/*.yml (on.merge_group)
status: active
description: >-
  A check required by the develop ruleset must carry a `merge_group:` trigger
  as well as `pull_request:`. Merge-queue entries are built on an ephemeral
  `gh-readonly-queue/...` ref, which fires `merge_group` and not
  `pull_request`, so a required check without that trigger never reports on
  the queue entry and the merge queue blocks until timeout.
---

## What it prevents

BUG-P3SXOL: `no-session-trailer.yml` triggered only on `pull_request`, but its
check was required on `develop`. Every queue entry sat in `AWAITING_CHECKS`
until `check_response_timeout_minutes` expired, requeued, and repeated —
nothing merged for hours while `origin/develop` stayed pinned.

The failure is silent in the same way BUG-CI7XKP was: CI is green on the PR
page and green on the queue run, because the missing check is *absent* rather
than failing. A required check that never reports is indistinguishable from
one that has not finished yet.

## Verifying it still holds

Presence, not absence — for every workflow backing a required check, assert
the `merge_group` trigger exists:

```bash
python3 -c "
import glob, yaml
for f in sorted(glob.glob('.github/workflows/*.yml')):
    d = yaml.safe_load(open(f))
    on = d[True] if True in d else d.get('on', {})
    if isinstance(on, dict) and 'pull_request' in on:
        print(('OK  ' if 'merge_group' in on else 'MISS'), f)
"
```

Any workflow listed `MISS` whose job name appears in the branch ruleset's
required checks will stall the queue. When adding a new required check, add
the `merge_group:` trigger in the same commit that makes it required.
