---
id: TKT-FP04ZE
type: ticket
title: 'Cut CI wall clock: de-serialize the build tail and fix Go cache thrash'
kind: enhancement
priority: medium
effort: s
status: done
---

CI took ~14 min wall clock. Two independent causes, both fixed.

## 1. A redundant compile gated the whole tail

The `build` job ran `go build ./cmd/rela` — a strict subset of what
Cross-Compile (linux/default) already builds — but `demos` and `docs` sat behind
`needs: [build]`, and `build` itself waited on all ten other jobs. That made a
redundant 60s compile the hinge of a three-stage serial tail: slowest job
(~520s), then build (60s), then demos (~215s).

Removed the job; `demos` and `docs` now start at t=0. Confirmed: `Docs` finished
75s into the run instead of ~9.5 minutes in. Code review independently verified
no job depended on it — all four `bin/rela` consumers build it themselves.

## 2. The Go build cache was thrashing

`setup-go` defaults to `cache: true` and does cache both `GOMODCACHE` and
`GOCACHE`. But it derives the key from the go.sum hash, which is identical for
all 15 Go jobs. So every concurrent job computed the same key, all missed, and
all but one failed to save with `Unable to reserve cache with key ... another
job may be creating this cache`.

The single winner was whichever job finished first, so the stored entry held
only that job's partial build graph. Observed sizes under one key ranged 10MB to
791MB. A run restoring the 10MB arch-lint entry into the Test job gained
essentially nothing.

The fix splits the two caches by their sharing properties, which is the part
worth remembering:

- **Module cache** (`~/go/pkg/mod`) is a function of go.sum alone, so it is
byte-identical in every job. It stays on setup-go's shared, go.sum-keyed entry.
Jobs still race to save it; the losers' warning is harmless because the winner
stored the same bytes.
- **Build cache** (`~/.cache/go-build`) genuinely differs per job, since each
compiles a different package set under different build tags. Only this gets a
per-job key.

A first attempt gave each job a per-job key covering *both* paths. That
projected ~12.6GB against GitHub's 10GB per-repo quota (measured: 10 populated
caches at 0.70GB average), and eviction is LRU and repo-wide, so the entries
would have continuously evicted each other — leaving CI colder than before, with
no visible symptom. See RR-AUWXN2.

## Also

- Cached the four version-pinned lint tools; the golangci-lint `go install`
alone was 61s per run.
- E2E Playwright workers were raised 2 -> 4 and then REVERTED. The revert is
  not because 4 is known bad: the E2E failure first blamed on it was a stale
  branch (7 commits behind develop), and a rebase went green. It is because 4
  was never independently justified, and because `internal/dataentry` builds
  its cmdexec runner per request with no `WithMaxConcurrent` (TKT-LP4EE8) —
  unlike `internal/transform`, which bounds conversions at 4. Bound that
  first. See RR-M292PZ.
- Dropped the `Build` required status check from the develop ruleset in the
same change. A required check that never reports leaves a merge-queue entry
queued forever (the failure mode of BUG-P3SXOL).

## Result

Verified on run 35337622282 (fully green, 23 jobs), compared like for like
against develop runs from the same week:

| | wall clock |
|---|---|
| develop (35326416365, 35327661277, 35332682574) | 865s, 934s, 976s (mean 925s) |
| this branch, green runs | 530s, 683s |

**26-43% faster.** The spread on both sides is dominated by `Postgres Backend`,
the noisiest job (499-682s across all six runs, unaffected by these changes).

Warm-cache effect per job, against the 875s baseline:

| Job | before | after |
|---|---|---|
| Lint | 258s | 142s |
| Demos | 256s | 121s |
| SQLite Backend | 219s | 101s |
| Frontend | 208s | 140s |
| Docs | 56s, after ~8.5 min of waiting | ran at t=0 |

## Deliberately not done

Caching the apt GTK4/WebKitGTK install, which costs 25–79s across 9 jobs (on
SQLite Backend the install is longer than the tests). Declining a third-party
action for this was the right default given how few this repo pins, but review
noted a dependency-free route exists — `apt-get install --download-only` into a
cached dir, then `dpkg -i`. That makes it a tier-2 item, not a blocked one.

Tier 2 candidates, unstarted: shard the Test job (503s single `go test`),
parallelize the three sequential Postgres test steps, move
`check-tagged-tests.sh` (124s) out of Demos, and extract the repeated cache
block into a local composite action (RR-SQ1AJR).
