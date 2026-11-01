---
id: TKT-VJ8FHJF
type: ticket
title: Validator loads candidates once per rule instead of once per type
kind: refactor
priority: low
effort: s
status: backlog
---

## Description

Found while measuring TKT-QM833U. `GenericValidator.CheckRuleFull`
(`internal/validator/validator.go`) loads the candidates of a rule's type for
every rule. `analyzeValidations` calls it once per rule, so atlas (78 rules)
lists and redacts 1,459 rows for 663 entities in one `_analyze` request.

After TKT-QM833U a reload costs one list page per rule, so the gain is about 60
statements and the redaction of about 800 rows per request on atlas.

## Approach

Group rules by entity type and load the candidates once per type, for example a
`CheckRulesFull(ctx, rules)` on the validator that returns results per rule.
Keep the rule order of the output.

Acceptance: `_analyze` issues one candidate list per entity type, not per rule;
output identical on the atlas-dev copy.
