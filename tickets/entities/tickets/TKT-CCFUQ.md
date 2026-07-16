---
id: TKT-CCFUQ
type: ticket
title: 'Close the =~ ReDoS hole: require trusted literal regex patterns (issue #1139)'
kind: enhancement
priority: medium
effort: s
status: done
---

Follow-up to RR-IROUO (rela#1137 / TKT-BL7XZ). Source: GitHub issue
[rela#1139](https://github.com/sourcehaven-bv/rela/issues/1139).

> **Note — the approach changed during review.** The issue proposed capping the
> matched value's length. That was implemented, then **disproved and abandoned**:
> a length cap cannot mitigate ReDoS. See RR-HPQV2. What shipped instead removes
> the untrusted-pattern path entirely. The issue's diagnosis (the mitigation was
> incomplete) was right; its proposed remedy was not.

## The real problem

`=~` in `frontend/src/utils/conditions.ts` runs `re.test()` on the render
thread. JS's RegExp backtracks with **no match timeout**. RR-IROUO capped the
*pattern* length (`MAX_REGEX_LENGTH=200`); issue #1139 correctly spotted that
this misses the attack, and proposed also capping the *value*.

**Measured — neither cap works.** A catastrophic pattern is *short* and blows up
on *tiny* inputs:

- `(a+)+$` vs a 41-char value: **hangs >60s** (pattern is 6 chars — under the
200 cap; value is 41 chars — 240× under any 10k value cap).
- `(a+)+$` vs a 27-char value: **~10s**.
- `(a+)+$` vs 100k all-`a`s: **0.11ms** — it *matches*; no backtracking. (This is
why the original regression test was hollow — see RR-P1QZK.)

Length caps only bound *linear* work. Neither cap could ever stop a hostile
regex.

## Threat model (the actual fix)

Two inputs meet at `=~`, unequally trusted:

| Input | Source | Trust |
|---|---|---|
| **Pattern** | `data-entry.yaml`, operator-authored | **Trusted** |
| **Value** | binding (`form.*`, `entity.*`) — user input | **Untrusted** |

RR-IROUO's own resolution already recorded patterns as *"trusted config,
defence-in-depth, not the boundary"*. So the only genuine vulnerability was a
**dynamic pattern** (`form.v =~ form.pat`) — the one path where a *user*
supplies the regex. Issue #1139 flags exactly this.

## What shipped

- **The parser rejects any non-literal `=~` pattern** (throws `ConditionError` at
parse). The untrusted-pattern path is *removed*, not mitigated. Being static, it
fails loud like every other config bug.
- **`compareRegex` simplified** — the pattern is now guaranteed to be a
parse-validated literal, so the dynamic-pattern cap and the compile try/catch
are dead code and are gone.
- **The value cap is kept but relabelled honestly**: hygiene bounding a *linear*
scan (a pasted megabyte on the render thread), explicitly **not** a ReDoS
boundary.
- **Threat model documented in the module doc**, including what this deliberately
does *not* fix.

## Accepted residual risk

An operator writing a pathological literal into their own YAML still hangs the
tab (~10s at 27 chars). That is a **foot-gun, not a vulnerability** — the same
operator can write `visible_when: false` — and it misfires in their own browser
while authoring, not a user's. Documented in-code with RE2 / Worker-timeout
named as the upgrade path **if `=~` ever needs a data-sourced pattern**.

## Grounds

POLICY-015 §3 (OWASP risks structurally addressed); CONTROL-8-28 / 8-29 (secure
coding / security testing).

## Timing

Engine is dormant (zero importers; no `visible_when`/`required_when` in any
config yet). The restriction lands **before** a consumer exists, so it costs
nothing now and cannot regress later. Note: the issue references **TKT-CHLAJ**,
which does not exist in this tracker.
