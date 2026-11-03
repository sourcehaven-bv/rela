---
id: RR-2ZYGVB
type: review-response
title: Holder cap counts raw property matches before field redaction
finding: 'Approach 4 says MatchingRefs needs no redaction "because only refs leave", and Approach 7.4 sets truncated when a holder query reaches inboxHolderCap. The pushed PropPredicate matches the RAW stored property, but `visible:` redaction is per row (FieldRedactor.HiddenProperties(ctx, e)), so it cannot be pushed. A caller whose role property is hidden on N rows still gets those N refs counted toward the cap. truncated then reveals "at least cap entities whose hidden property names me". That is a value disclosure, not the accepted membership bit, and AC 8 ("at the holder cap", hidden-property kind) would fail on this design. Fix: inside MatchingRefs, redact each streamed row (RedactRow, as listPushdown''s redactingSeq does), re-apply the Go prop test (string or list element equality) to the redacted row, and count only rows that survive toward limit+1. The same applies to the no-pushdown fallback. State in the godoc that the cap counts rows after redaction.'
severity: critical
resolution: HolderLookup redacts each streamed row and re-applies the Go prop test before counting toward limit+1; fallback does the same. Godoc says the cap counts after redaction. AC 5 adds TestHolderLookup_HiddenPropertyNotCounted.
status: addressed
---
