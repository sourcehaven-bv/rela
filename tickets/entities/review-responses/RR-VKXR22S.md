---
id: RR-VKXR22S
type: review-response
title: Relation-leaf limit hit by per-leaf visibility rewrite
finding: Rewriting each leaf on a hidden property copies V once per leaf; local-role policies push the tree past MaxCondRelations=16. Rewrite once per maximal same-property subtree.
severity: significant
resolution: guardTree guards the largest same-property subtree once and guardHiddenProps groups props per property; store.ValidateCond runs on the rewritten tree and an over-limit tree is refused with ErrPropsNotPushable. TestGuardHiddenProps pins one guard per subtree.
status: addressed
---

## Finding

Rewriting each leaf on a hidden property copies V once per leaf; local-role
policies push the tree past MaxCondRelations=16. Rewrite once per maximal
same-property subtree.
