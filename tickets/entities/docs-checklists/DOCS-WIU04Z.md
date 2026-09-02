---
id: DOCS-WIU04Z
type: docs-checklist
title: 'Docs: Warn when unmatched_principal reject has no JWT gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

`warnUnmatchedRejectWithoutJWTGate` is unexported but carries a full doc
comment, because three things about it are non-obvious:

- **Why it lives in `NewRouter` and not `Policy.Validate`.** Validate sees
`acl.yaml`; whether a JWT gate is wired is a property of the SERVER, decided
elsewhere and later. `NewRouter` is the first point where both facts are in
scope. Without this note the natural instinct is to "tidy" the check in with the
rest of the policy validation, where it cannot work.
- **Why it warns rather than fails.** Refusing to start turns a wiring omission
into an outage for a deployment that is merely no stricter than the default. A
reader who thinks "security check, should be fatal" needs the counterweight in
front of them.
- **That it also gives the SetJWTGate/NewRouter ordering invariant a runtime
voice.** That invariant was previously protected only by a code comment, so a
refactor reordering them failed silently. It now warns. Recorded because the
check is doing two jobs and only one is in the ticket title.

The test file's header comment states the reasoning the tests encode — why a
warning and not an error, and why the `NewRouter`-level test exists at all when
four others already cover the predicate.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: no new pattern — it reuses
the `slog.Warn` channel the `provision` mode already uses for the same class of
problem)
- [x] docs/ updated for changed behaviour — see below
- [x] ~~Architecture docs updated~~ (N/A: no package boundary, dependency or
wiring-contract change)

`docs/acl-security.md` gains a bullet in the `unmatched_principal` section's
"Load-bearing details" list, alongside the existing entries for the lookup
requirement and the provisioner grant.

The doc already said `reject` "keys on the fact that identity came from the
fail-closed JWT gate" — true, and not enough: it describes the mechanism without
saying what happens when there is no gate. The new bullet closes that, states
explicitly that the key has **no effect** in that case, explains why it is a
warning rather than the load error the missing-lookup case gets (`acl.yaml`
cannot see server wiring), and tells an operator who configured `reject` and
sees writes going through to check that warning first.

That last sentence is the practical one. The failure this ticket addresses is
someone believing a restriction is in force; the doc is where they will look
when they eventually notice it is not.

Edited in `docs-project/entities/guides/GUIDE-acl-security.md` and regenerated
with `just docs`. `docs/` is GENERATED — editing the output directly fails the
Docs CI check, which is exactly how the sibling ticket TKT-LVSPSB failed CI
earlier in this same issue round.

## External Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLI reference updated~~ (N/A: no command or flag changes)
- [x] ~~API docs updated~~ (N/A: no HTTP surface change — no route, request or
response differs. The change is a startup log line)

## Rationale for N/A

Nothing an API consumer can observe changes: no authorization decision is
altered, only reported. A deployment with `reject` and no gate behaved as
`anonymous` before this change and behaves as `anonymous` after it — the
difference is that it now says so.

The two audiences are covered separately and deliberately. The warning reaches
an operator who has ALREADY deployed the misconfiguration; the doc bullet
reaches one who has not yet. Either alone leaves half the problem: a warning
nobody knows to expect reads as noise, and a doc nobody re-reads does not help
someone already running.
