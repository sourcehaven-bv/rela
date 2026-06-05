---
id: BUG-6PYB6G
type: bug
title: Audit log has no documented 12-month retention policy; docs suggest non-compliant 90-day cleanup
description: |-
    The append-only JSONL audit log under `.rela/audit/` rotates daily but has no documented retention policy, so ≥12-month retention (POLICY-017 §4 / PROCEDURE-f4cu) was not demonstrable. Worse, `docs/audit-log.md` actively suggested a **non-compliant** cleanup: `find .rela/audit -mtime +90 -delete` (90 days = 3 months). An operator following the docs would delete security logs they are required to keep.

    **Key architectural fact:** `rela` itself **never deletes audit logs** — the backend only rotates to a new `YYYY-MM-DD.jsonl` each UTC day and appends; there is no pruning/expiry code path. So the application is retention-safe by default and cannot, on its own, drop a record below any window. Retention is an **operational control** owned by the deployment, not a missing application feature. The only compliance risk was the misleading documentation.

    **Fix (docs + a CI guard against doc regression):**
    - `docs/audit-log.md`: replaced the `-mtime +90` example with explicit ≥12-month guidance and an `-mtime +365` example that prunes only beyond the retention window; stated that rela never auto-deletes and that the dir is gitignored/per-machine (back it up off ephemeral hosts).
    - `docs/security.md` (the system security/architecture spec): added a **Retention** subsection under "Audit logging" documenting the POLICY-017 ≥12-month requirement, the never-deletes guarantee, and that enforcement is operational.
    - `scripts/check-audit-retention-docs.sh`: a CI guard (wired into the Lint Markdown job) that fails if `docs/audit-log.md` reintroduces a `find … -mtime +N` cleanup with N < 365, so the misleading example can't silently come back.
priority: medium
effort: s
why1: There was no documented retention policy for the audit log, so ≥12-month retention could not be demonstrated for compliance.
why2: The docs' Retention section offered a `-mtime +90 -delete` example, which not only failed to state the requirement but actively pointed operators at a 3-month cleanup that violates it.
why3: The audit backend was built to rotate-and-append with no deletion, so retention was implicitly "forever" — but that guarantee was never written down, and the docs contradicted it.
why4: Retention is an operational control (backup/log-shipping/filesystem policy), and rela had no convention for stating which compliance obligations the deployment owns vs. the application — so the obligation went undocumented.
why5: There was no automated guard on compliance-relevant documentation, so a well-intentioned "here's how to clean up disk" example could ship and stand without anyone noticing it conflicted with a retention policy.
prevention: |-
    `scripts/check-audit-retention-docs.sh` runs in the Lint Markdown CI job
    and fails the build if `docs/audit-log.md` ever again documents a
    `find … -mtime +N` audit cleanup with N < 365 days. Verified: it passes
    on the corrected docs and fails on a reintroduced `-mtime +90` example.

    The retention requirement is now recorded in two places a reviewer/auditor
    will look: the operator-facing `docs/audit-log.md#retention` and the
    security/architecture spec `docs/security.md#retention`. The application's
    never-delete behaviour is the durable guarantee; the docs make the
    operational obligation explicit.
status: done
---

See GitHub issue #887.
