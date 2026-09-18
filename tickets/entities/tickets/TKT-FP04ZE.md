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
- Raised E2E Playwright workers 2 → 4 on a 4-core runner (E2E 409s → 361s).
Per-test state is isolated: ephemeral port, `mkdtemp` project dir, and a
`relae2e_<pid>_<n>` postgres schema. The postgres container and the CPU are
*not* isolated, and `retries: 2` would mask load flake as a slow green — the
config comment says so rather than claiming blanket safety (RR-M292PZ).
- Dropped the `Build` required status check from the develop ruleset in the
same change. A required check that never reports leaves a merge-queue entry
queued forever (the failure mode of BUG-P3SXOL).

## Result

875s → 530s (39%), measured on a cold cache, so that is the floor rather than
the steady state.

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
