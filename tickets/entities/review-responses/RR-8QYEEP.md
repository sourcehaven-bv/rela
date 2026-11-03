---
id: RR-8QYEEP
type: review-response
title: Duplicate-label refusal blocks the per-role same-label pattern
finding: 'AC 5 refuses two equal preset labels in one state. A natural config is [{to: fast-track, roles: [lead], button: Approve}, {to: approved, button: Approve}]. The second entry is shadowed for a lead and fires for everyone else, so no single caller sees both buttons. Refuse a duplicate only when some person class reaches both entries, and otherwise allow it. Also apply texthazard.DropStyleSelectors and NFC before trimming and case folding, as normalizeListItem does (internal/realtime/validate.go:193), so that two labels that look the same compare equal.'
severity: minor
resolution: Duplicate labels are now allowed when no single caller sees both, which holds when both entries share a type and the earlier one shadows the later. Labels compare after DropStyleSelectors, NFC, trim and case folding.
status: addressed
---
