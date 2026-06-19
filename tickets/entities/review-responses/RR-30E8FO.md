---
id: RR-30E8FO
type: review-response
title: ApplyEntity skips status defaulting — precondition undocumented/untested
finding: CreateEntity defaults status when empty (core.go:154-156) before validating; ApplyEntity does not, and suppresses automation, so nothing backfills. A peer emitting a status-less entity persists status-less. The 'caller owns complete state' assumption was asserted nowhere (all tests passed explicit status).
severity: significant
resolution: 'Documented the precondition in the ApplyEntity godoc (''PRECONDITION: the caller must supply every field including status — there is no backfill''). Pinned the chosen behavior with TestApplyEntity_NoStatusAppliesAsIs: an entity supplied without status persists without one (no silent defaulting). This is correct for a sync mirror — the origin owns defaults.'
status: addressed
---
