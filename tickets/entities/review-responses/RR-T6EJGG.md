---
id: RR-T6EJGG
type: review-response
title: Server-read-only gate means a needs-migration store serves indefinitely with no operator signal
finding: 'Making the server read-only is right for avoiding git-tracked writes at boot, but the plan does not say what a server does when it evaluates needs-migration. Today the gate at least logs and the operator is expected to run the CLI. With no write and only a log line, a server can run indefinitely against data that does not fit the schema, with writes soft-failing per DEC-HWZHA. The plan needs an explicit answer: refuse to boot, expose readiness/health, or surface it in the UI. Also unspecified: whether the conservative bootstrap refusal applies to the server (which cannot run the baseline command) or only to the CLI.'
severity: significant
resolution: 'Resolved conservatively. (1) needs-migration does NOT become fatal at server boot in this ticket: that is a separable behaviour change for existing deployments and belongs in its own ticket alongside TKT-5RAW5I, which makes the same invisible-log-line argument for the drift tier. (2) The verdict IS surfaced beyond the startup log, through the existing server status/health surface, so it is discoverable without grepping logs. (3) The conservative bootstrap refusal applies to the CLI only; a server with no recorded state evaluates and serves rather than refusing, because it cannot run the baseline command and a clone-and-deploy flow must not deadlock. Recorded in the plan''s Approach.'
status: addressed
---

## Finding

"CLI writes, server read-only" is the right call for avoiding surprise writes to
a git-tracked file, and it cleanly removes the multi-instance adoption race. But
the plan does not specify **what the server does when its evaluation says
needs-migration**, and the answer is load-bearing.

Today the gate warns (`gate.go:129-136`) and the operator is expected to notice
and run the CLI. With persistence removed, a server can now run **indefinitely**
against data that does not fit the live schema, because:

- nothing is written, so there is no state change to notice;
- writes stay soft (DEC-HWZHA keeps them non-blocking either way);
- the only signal is a startup `slog.Warn` — and TKT-5RAW5I already establishes
that startup log lines in a healthy process are operationally invisible.

This is the same critique TKT-5RAW5I makes of the drift tier, applied to the
*more* severe tier. Needs-migration means stored values genuinely do not fit.

## Unanswered questions the plan must settle

1. **Does the server refuse to boot on needs-migration?** Rails' analogue
(`PendingMigrationError`) does exactly that, and the research found it is the
conventional, well-regarded behaviour. Today rela does not, and changing it is a
behaviour change for existing deployments.
2. **Is it exposed anywhere a human looks?** A health/readiness endpoint or a UI
banner is the difference between "operationally invisible" and "actionable".
3. **Does the conservative bootstrap refusal apply to the server?** A server
cannot run the baseline command, so if refusal applies there, a fresh deployment
from a clone with a non-empty `migrations/` will not start until someone runs
the CLI. That may be correct (it is exactly the case where the data's shape is
unknown) but it must be a decision, not an accident.

## Recommendation

Keep the server read-only. Add to the plan: the server **surfaces** the verdict
rather than just logging it — at minimum through the existing health/status
surface — and state explicitly whether needs-migration is fatal at boot.
Recommend NOT making it fatal in this ticket (it is a separable behaviour change
with its own migration story for deployments) but recording it as the open
question it is.

The bootstrap-refusal-on-server question needs an answer before implementation
because it determines whether a clone-and-deploy flow works at all.

## Evidence

- `internal/datamigration/gate.go:129-136` — needs-migration warn path
- `internal/appbuild/datamigration.go:44-48` — gate failures degrade to a warning
and must never fail boot
- TKT-5RAW5I — the same invisible-log-line critique for the drift tier
- Research: Rails `PendingMigrationError`, Django `migrate --check` both refuse
