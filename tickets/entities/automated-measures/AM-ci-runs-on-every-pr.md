---
id: AM-ci-runs-on-every-pr
type: automated-measure
title: "CI and CodeQL trigger on every pull request, whatever branch it targets"
kind: ci
location: .github/workflows/ci.yml + .github/workflows/codeql.yml (on.pull_request)
status: active
description: >-
  The `pull_request` trigger in ci.yml and codeql.yml carries no `branches:`
  filter, so every PR is built and scanned regardless of the branch it targets.
  Push triggers keep their filters; the PR trigger deliberately has none,
  because a PR is a request to merge code whatever it is stacked on.
---

## What it prevents

BUG-CI7XKP: a `branches: [main, develop]` filter on `pull_request` matches the
**target** branch, so a stacked PR — one opened against another feature branch
— matched no workflow and ran zero checks. A new authenticated HTTP endpoint
was reviewed and approved that way, with no SAST, no SCA and no tests having
run on its branch.

The failure is silent by construction. A PR with no checks looks the same as a
PR whose checks all passed: no red on the page, and `gh pr checks` reports
success because nothing failed. Nothing else in the repo can catch it, because
every other guardrail lives *inside* a workflow that never started.

## Why the filter is not restored

The obvious objection is cost — building branches nobody intends to merge. But
the PR trigger only fires when a pull request exists, which is precisely the
moment the code is proposed for merge. The push trigger, which is where
build-everything cost would actually accumulate, keeps its `branches:` filter.

## Verifying it still holds

Presence, not absence:

```bash
# Must print `pull_request: None` (unfiltered) for both files.
python3 -c "
import yaml
for f in ['.github/workflows/ci.yml','.github/workflows/codeql.yml']:
    d = yaml.safe_load(open(f))
    on = d[True] if True in d else d['on']   # YAML 1.1 parses bare 'on' as True
    print(f, '->', 'pull_request:', on['pull_request'])
"
```

When checking a PR's CI, assert the required jobs are **present**. A filter on
`.bucket == "fail"` returns empty both when everything passed and when nothing
ran, so it cannot distinguish the two:

```bash
gh pr checks <N> --json name,bucket |
  jq -e 'any(.name|test("CodeQL")) and any(.name|test("Vulnerability"))'
```
