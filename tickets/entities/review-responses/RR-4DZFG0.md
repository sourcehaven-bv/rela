---
id: RR-4DZFG0
type: review-response
title: Scope error locator differed from other card errors
finding: checkQueryScopeRef was called with prefix/title instead of the card[%d] locator.
severity: nit
resolution: 'The message is prefixed with the card locator. Test case: scope error names the card.'
status: addressed
---
