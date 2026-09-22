---
id: REV-CAOV03
type: review-checklist
title: 'Review: reverse_relation migration step: rewrite stored edges when a relation type''s direction is swapped'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — plus the postgres and sqlite suites, which
      this change touches: `go test -tags postgres ./internal/store/pgstore/`
      and `-tags sqlite ./internal/store/sqlitestore/` both green
- [x] Lint clean (`just lint`) — 0 issues
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) — thresholds PASS
- [x] `just arch-lint` clean — the store capability adds no application
      dependency to a store package

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5XNMEN, RR-EL513R, RR-U31Q7Q (critical, all addressed);
RR-BC2PPF, RR-L5S5VG (significant, both addressed); RR-G72VZX (minor,
addressed); RR-2TPCTX, RR-3QWH45 (minor, deferred with reasons).

Design-review findings from the planning phase: RR-6U41C3, RR-2IKYBT,
RR-EHKYDB, RR-RN0QB8, RR-838OB8, RR-0MQ4ZF, RR-2L6XJB, RR-ASNIG5, RR-A7DK8Q.

The three criticals were all real and all confirmed before fixing:

- The swap did not bump `updated_at`, so the version sweep might never select
  the rewritten row — the TKT-9TQ6I trap, whose "a miss costs only the marker"
  exemption does not transfer, because nothing captures a reversal
  synchronously. Fixed in both backends, pinned by a conformance case, and
  mutation-verified.
- The fallback's collision mirror was face-blind. Verified directly: a stored
  edge keys as `A@draft--blocks--B` while the constructed mirror keys as
  `B--blocks--A`. Unreachable today, which made it worse rather than better —
  the store depended on a caller-side guarantee it could not itself check. The
  store now asserts the precondition instead of trusting it.
- Three backends emitted three different event streams for one operation, and
  the conformance suite was green throughout because it only compared rows.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC-0 (both paths, one contract) — PASS. `RunBulkMigrateTests` runs
  unconditionally on fs, memory, sqlite and postgres.
- AC-1 (rewrites every edge, keeps properties and body) — PASS, unit and
  end-to-end.
- AC-2 (re-run does not flip back) — PASS. The marker records the applied file;
  the store operation itself is a mechanical swap with no memory, which the
  suite asserts explicitly so nobody adds a hidden marker below the schema.
- AC-3 (content-scoped refused before any write) — PASS, now refused by the
  store itself as well as the step.
- AC-4 (symmetric refused) / overlapping endpoints refused — PASS at parse time.
- AC-5 (one delta, not two narrowings) — PASS.
- AC-6 (generator drafts a live step) — PASS; the draft parses.
- AC-7 (file spanning the delta must carry the step) — PASS both ways, and
  mutation-verified against the subject-prefix trap the design review predicted.
- AC-8 (cardinality warning) — PASS, and now reported once per bound pair.
- AC-9 (all-or-nothing on a collision) — PASS; both edges survive a refusal.
- AC-10 (lineage preserved on a versioning backend) — PASS, mutation-verified
  by swapping the in-place UPDATE for a DELETE+INSERT.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-GDJHQB

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
