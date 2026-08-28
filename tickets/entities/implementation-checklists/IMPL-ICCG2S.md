---
id: IMPL-ICCG2S
type: implementation-checklist
title: 'Implementation: DisplayTitle bypasses the hidden-primary-property fallback on four surfaces (views, mentions, analyze, settings)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — 5 tests across mentions/settings/analyze.
- [x] Integration tests written — settings + analyze drive their real HTTP handlers end-to-end through the gate.
- [x] Happy path implemented — all three surfaces route targets through `viewReader.Filter` / `visibility.Redact`.
- [x] Edge cases handled — unreadable target (dropped) AND readable-but-hidden-title (id fallback) covered on each surface; mentions context-cancellation preserved.
- [x] Error handling in place — `Filter` fails closed on gate error (logged); analyze load-miss treated as not-hidden (no title to leak).

## Test Quality

- [x] Using fixture builders or factories — `seedEntity` / `mustNewACL` / `gateCtxFor` / `mustAllowAll`.
- [x] ~~No hardcoded values in assertions~~ (N/A: sentinel leak-checks require a literal probe string by design).
- [x] Only specifying values that matter — a hidden property + a distinctive value; nothing else.
- [x] ~~Interpolated values constructed from objects~~ (N/A: sentinel leak check).
- [x] ~~Property comparisons use original object~~ (N/A: sentinel leak check).

## Manual Verification

- [x] Feature manually tested end-to-end — neutralized each fix, confirmed the matching test fails (leak reproduced), restored, confirmed pass.
- [x] Each acceptance criterion verified — hidden title absent + unreadable target dropped, per surface.
- [x] ~~Edge cases manually verified~~ (N/A: covered by the neutralize/restore round-trips and the existing suite).

**Verification Evidence:** For each surface, the fix was neutralized and the
matching test FAILED with the sentinel (`SECRET-MENTION` / `SECRET-TARGET` /
`UNREADABLE-TARGET` / `SECRET-ANALYZE-TITLE`) visible; restored, all pass. Full
`internal/dataentry` + `internal/visibility` green under `-race`;
`golangci-lint` 0 issues; `just plimsoll` + `just arch-lint` clean.

## Quality

- [x] Code follows project patterns — same `viewReader.Filter` / `Redact` seam as the #1212 view fix (DEC-ZBI39P).
- [x] Checked for DRY opportunities — reused `appRedactor`; `hiddenDisplayTitleEntityIDs` is a package func (not an App method) to stay under the plimsoll cap.
- [x] No security issues introduced — the change removes disclosures; every path fails closed.
- [x] No silent failures — gate errors logged; the one deliberate load-miss swallow is documented (no title to leak).
- [x] No debug code left behind — all neutralizations reverted.
