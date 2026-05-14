---
id: RR-W5FU
type: review-response
title: Eat-range probe uses ch+1 ignoring UTF-16 surrogate pair boundaries
finding: 'useBacktickAutocomplete.ts line 541-542 reads `nextChar = getRange(cursor, {ch+1})` to detect an auto-paired backtick. CodeMirror positions are UTF-16 code units, so for an astral-plane character at cursor (rare in technical writing but not impossible — think emoji), `ch+1` lands inside a surrogate pair and returns a lone surrogate. The comparison `nextChar === ''`''` then fails (good — it''s not a backtick), but the subsequent `replaceRange('''', trig, eatRangeEnd)` with `eatRangeEnd = cursor` is fine because that branch eats only the trigger→cursor range. The actual hazard: if `nextChar` happens to be the lone-surrogate prefix of a paired closing-backtick scenario (won''t happen with `‘''’`-char composition, but defensive code style would matter for future-proofing). Practical impact: very low. Nit-level. Fix: use getRange with codepoint-aware boundaries, or treat any non-ASCII nextChar as a non-match (already does).'
severity: nit
reason: UTF-16 surrogate pair handling at the pick site is a theoretical edge case (entity IDs never contain non-BMP characters). storeutil.ValidateID would reject any ID that did. Defer.
status: deferred
---
