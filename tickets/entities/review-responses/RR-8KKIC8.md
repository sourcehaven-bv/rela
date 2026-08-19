---
id: RR-8KKIC8
type: review-response
title: Wrong-side hint became unreachable dead code
finding: 'The `(set direction: X to bind the Y side)` hint was gated on r.Direction == "" plus entityType in the opposite set — but once inference resolves an absent direction, that combination returns nil earlier, so the hint could never fire. The reviewer brute-forced every from/to/entityType combination to confirm unreachability. The test that used to cover it had been rewritten and no longer asserted the hint, so user-facing error text was unexecuted and untested.'
severity: significant
resolution: 'Inverted the gate to r.Direction != "": the hint now fires where it is genuinely actionable — the author wrote an explicit direction and picked the wrong one. For an absent direction no hint is possible (inference already resolved it, so a wrong-side error means the type is on neither side and flipping would not help); the godoc says so. TestValidateConfig_FormRelationWrongSide_HintsWhenDirectionExplicit now asserts the hint text.'
status: addressed
---
