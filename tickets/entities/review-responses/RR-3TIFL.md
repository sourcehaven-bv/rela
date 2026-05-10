---
id: RR-3TIFL
type: review-response
title: rela validate already exists — strict-validation alternative is wrong
finding: 'Plan''s Out-of-scope and Alternatives sections both reject adding strict-validation surfaces, claim ''scripts call analyze tools'' or ''check warnings explicitly''. Verified: internal/cli/validate.go exists. CI scripts calling ''rela validate'' continue working post-ticket. Plan should reference this as the strict-validation entry point, not pretend the question was hand-waved. Lua side is genuinely missing rela.validate but should be a documented follow-up, not an implicit no. From design-review F4.'
severity: significant
resolution: Verified internal/cli/validate.go exists. Plan's Out-of-scope section now references rela validate as the strict-validation entry point for CI scripts. Lua-side rela.validate noted as follow-up but not in scope for this ticket. Alternatives section updated to acknowledge rather than dismiss the question.
status: addressed
---
