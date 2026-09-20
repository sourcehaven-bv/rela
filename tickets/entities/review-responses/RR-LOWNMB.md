---
id: RR-LOWNMB
type: review-response
title: Incoming-edge partial-failure design solves a problem that does not exist
finding: 'The ticket asserted twice (Flow step 5, and a ''Partial failure on incoming edges'' risk) that the batched create covers link_as: to only, so each incoming edge is a separate call that can fail after the entity exists. This is false. resolveDirection (relations_direction.go:41-57) maps an inverse body key back to its canonical relation and flags it incoming; the apply path (relations_modern.go:274, :61) honours that, and relationsPatch.ts:50-53 documents the client side. Incoming edges ride the single batched create atomically under inverse-named keys. Implementing AC17 as written would have added a post-create call path that does not need to exist, introducing the very non-atomicity the plan feared.'
severity: significant
resolution: 'AC17 and the partial-failure risk deleted. Flow step 5 corrected: the batched create writes both directions in one call via resolveDirection, and the client passes relation keys through unchanged.'
status: addressed
---

## Resolution

AC17 and its risk entry are deleted; Flow step 5 is corrected to state that the
batched create writes both directions in one call and that the client passes
relation keys through exactly as the read endpoint returned them.

The likely origin of the error: `InlineCreateFormModal.vue:52-54` says "Only
`linkAs: 'to'` is applied here... The reverse direction is the host's job" — but
that is about that component's single pre-link prop, not a limit of the create
body.
