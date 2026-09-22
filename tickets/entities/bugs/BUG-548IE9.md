---
id: BUG-548IE9
type: bug
title: 'Two ticket-status gates disagree: a blocked item passes one and fails the other in the same CI run'
description: |-
    .github/workflows/ci.yml accepted `blocked` as a mergeable terminal state while tickets/schema.yaml's ci-no-blocked-tickets and ci-no-blocked-bugs reject it at severity error. A blocked item therefore passed the Ticket done-before-merge check and failed the validation rules in the same run, with no status satisfying both. Separately, `ready` is rejected by the job and permitted by the rules, which is correct but was undocumented, so a clean local `rela validate` did not predict the job's verdict.
priority: medium
effort: s
why1: A bug or ticket at `blocked` is waved through by the ci.yml status gate and failed by ci-no-blocked-* in the same CI run.
why2: The job's accept list named `blocked` as terminal while the schema rules called it unmergeable; nothing reconciled the two lists.
why3: The job's own comment says the ci-no-* rules "already block planning / in-progress / review / analyzing / blocked" — so the author knew those rules covered blocked, yet still listed it as accepted one line below.
why4: The job was written to close a specific hole (a `ready` item merging alongside its code, how TKT-QXHFJZ shipped stale). Its accept list was written as "states that are not mid-flight" rather than derived from the rules it cites, so it drifted from them at the one status where the two ideas differ.
why5: 'Systemic: the same policy is expressed twice, in two languages, in two files, with no mechanism forcing them to agree. The two are not even the same shape — the rules are corpus-wide and the job is diff-scoped — so a divergence can be correct (ready) or a contradiction (blocked), and nothing distinguishes those cases. Agreement was maintained by whoever edited one remembering the other.'
prevention: 'P1 (implemented): `blocked` removed from the job accept list, so both gates reject it, in the direction ci-no-blocked-* states ("resolve or revert"). P2 (implemented): `blocked` gets its own branch and message in the job rather than falling to the generic arm, so the failure says what to do. P3 (implemented): the scope difference is documented at the CI Gates block in schema.yaml — corpus-wide rules versus diff-scoped job — with `ready` named as the deliberate divergence and the reason a ci-no-ready-* rule must NOT be added. P4 (not done, follow-up): derive one list from the other, or add a test that fails when they disagree on a status neither file marks as intentionally scoped. That is the only fix that makes drift impossible rather than merely documented.'
status: backlog
---

## The contradiction

`.github/workflows/ci.yml` accepted four statuses as terminal:

```
done|wont-fix|blocked|backlog) ;;
```

`tickets/schema.yaml` rejects one of them:

```yaml
- name: ci-no-blocked-bugs
  description: "CI: Bugs in 'blocked' status cannot be merged - resolve or revert"
  when: ["status=blocked"]
  then: ["status!=blocked"]
  severity: error
```

Verified by measurement, not by reading: setting a bug to `blocked` and running
`rela validate --check validations` fails with *"CI: Bugs in 'blocked' status
cannot be merged"*, while the job's `case` waves the same entity through.

Nobody had hit it because nothing had tried to merge at `blocked`.

## Why `ready` is NOT the same defect

The two gates have different scopes, and for `ready` that difference is load-bearing:

| | scope | `ready` |
|---|---|---|
| `ci-no-*` rules | whole corpus, every run | must pass — 12 ready items exist on develop |
| done-before-merge job | only entities the PR changed | must fail — a touched ready item means a stale ticket |

A `ci-no-ready-*` rule would fail CI on all twelve. The schema's own comment
says work must be "not started (backlog/ready) or fully complete", so `ready`
existing is by design; only `ready` *merging alongside its own code* is the
defect, and only the diff-scoped job can see that.

So `ready` stays enforced in exactly one place. What was missing was any record
of why, which is what let it read as a second contradiction.

## Consequence

A clean local `rela validate --project tickets` did not predict the Rela
Tickets job's verdict. Three sessions hit this in one day, including on
PR #1657, where BUG-RGVKRV was filed at `ready`, passed validation locally and
failed the job.
