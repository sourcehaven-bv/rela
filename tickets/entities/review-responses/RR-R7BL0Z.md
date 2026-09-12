---
id: RR-R7BL0Z
type: review-response
title: 'Negation scope lost: !(alpha || beta) reported alpha and beta as opt-in tags'
finding: 'scripts/check-tagged-tests.sh: discovery ran `tr ''(),|&'' '' ''` to split terms, which strips parens BEFORE anything inspects the `!`. The negation is orphaned from the group it negates, the bare `!` is dropped by the identifier allowlist, and both tags survive. Go''s semantics are the opposite: such a file builds by DEFAULT and is EXCLUDED under -tags alpha. So the guard claimed to cover `alpha` while the vet it ran specifically excluded the only file mentioning it — a coverage claim it cannot honour. `//go:build ! alpha` (bang, space) had the same defect.'
severity: critical
resolution: 'Fixed by the same move to go/build/constraint. The collect() walker tracks negation depth and records only operands under an even number of negations, so scope is preserved structurally rather than textually. Cases added: "negated group yields no tags", "bang-space negation yields no tags", and "double negation is an opt-in tag" to pin the even/odd rule rather than just the common case.'
status: addressed
---
