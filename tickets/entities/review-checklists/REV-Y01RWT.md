---
id: REV-Y01RWT
type: review-checklist
title: 'Review: ticket validation silently passed over unparseable entities'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks
- [x] `rela validate --check cardinality --check properties --check validations`
passes on the tickets project (was 10 errors)
- [x] Guard step verified to FIRE on an injected parse error, and validation
verified to report "All validations passed" on that same tree — the silent-pass
this ticket exists for, reproduced directly
- [x] `just docs` regenerated `docs/mcp-server.md`; generated output matches the
source guide edit
- [x] Workflow YAML parses; step ordering and `if:` conditions inspected

## Code Review
- [x] Self-reviewed the diff
- [x] Guard ordered BEFORE validation, so a parse error is never masked by a
green validation result
- [x] New Lua rule verified in all three branches: parent backlog (passes),
parent flipped to in-progress (fails, naming the parent and its status), parent
relation removed (fails as orphaned)
- [x] No `${ }` interpolation of untrusted input added to the workflow; the
new step uses shell env vars only

## Acceptance Verification
- [x] All 10 pre-existing violations resolved
- [x] Backfilled checklists labelled as reconstructed and claim no new
verification — except TKT-UIR41P's docs, which were genuinely missing and were
written rather than waved through as N/A
- [x] `ci-no-open-review-responses` now models intent: a finding may stay open
while its parent is backlog/ready, so design-review output on unstarted work no
longer forces a misleading status

## Documentation
- [x] `GUIDE-mcp-server.md` — new "Read gating (ACL)" section (source of truth);
`docs/mcp-server.md` regenerated
- [x] Rationale for each guard recorded as comments at the point of change
- [x] Ticket records the root cause and the two independent CI holes

## Final Checks
- [x] Commit messages explain the why
- [x] Ready to merge

## Pull Request
- [x] PR opened against develop.
