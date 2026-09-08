---
id: RR-HI9QIU
type: review-response
title: 'Retry loop is unreachable: entitymanager converts UniquePropertyError into a validation error with no wrapped cause'
finding: 'The whole conflict-detection design (find -> create -> loser re-finds) never fires on the create path. isWebhookConflict (webhook_routes.go:502-508) tests errors.As(err, &store.UniquePropertyError) and errors.Is(err, store.ErrConflict). But entitymanager.createCore (core.go:127) calls mapUniquePropertyConflict, which (unique.go:136-151) returns newValidationError([]*metamodel.ValidationError{{Type: ValidationErrorUnique, ...}}) — a FRESH error carrying no wrapped cause. errors.As therefore cannot see the store error, isWebhookConflict returns false, and runPipeline takes the !isWebhookConflict early-return at :214 and surfaces a 500. Measured by the reviewing agent through the production router against live PostgreSQL: 8 concurrent creates on one unique value — 1 winner, 8/8 LOSERS GOT 500, zero retries, zero re-finds. The loop at :205-226 is dead code on this path. Consequence for the motivating use case: two HA Icinga masters sending byte-identical notifications produce one incident plus one 500, and since Icinga never retries, that alert is lost — exactly the failure the conditional-write design was chosen to prevent. Note the docs and code comments describe the intended behaviour accurately; only the error plumbing is wrong, which is why it reads as correct. Fix: either have mapUniquePropertyConflict wrap the store error (%w) so errors.As still resolves, or have isWebhookConflict additionally recognise a metamodel.ValidationErrorUnique validation error. The first is better — it repairs the seam for every caller rather than teaching one consumer about a leaky mapping.'
severity: critical
resolution: 'Fixed in two parts. (1) entitymanager.ValidationError gained an unexported cause field and an Unwrap method, and mapUniquePropertyConflict now uses newValidationErrorFrom(err, ...) so errors.As still resolves store.UniquePropertyError through the 422 re-presentation. (2) The root cause turned out to be broader than the store path: entitymanager''s PRE-WRITE scan (checkUniqueProperties) raises its own ValidationError with no store error to wrap at all, and that is the path most conflicts actually take. isWebhookConflict now also matches a metamodel.ValidationErrorUnique inside a *entitymanager.ValidationError — deliberately narrow, so an ordinary validation failure (which re-running would reproduce forever) is still NOT retried. Verified end to end: the rewritten TestWebhookConflict_LoserRefindsAndProceeds failed against the old code with 7/8 losers getting 500, then 409 after part (1) alone (conflict detected, retries exhausted), and passes after part (2) with all 8 deliveries returning 200 and reporting the same entity id.'
status: addressed
---

The design is right and the code *describes* it correctly; the error plumbing
defeats it.

webhook isWebhookConflict:  errors.As(err, &store.UniquePropertyError) ^ cannot
match entitymanager createCore:   mapUniquePropertyConflict(err)
|
entitymanager unique.go:    return newValidationError(...)   <- no %w

`newValidationError` constructs a fresh error from a
`[]*metamodel.ValidationError`. Nothing wraps the original
`store.UniquePropertyError`, so `errors.As` fails, `isWebhookConflict` returns
false, and `runPipeline` takes its `!isWebhookConflict` early return — a 500.

**Measured, not inferred**: 8 concurrent creates on one `unique:` value through
the production router against live PostgreSQL gave 1 winner and **8/8 losers a
500**, with zero retries and zero re-finds.

For the motivating case this is the exact failure the design was chosen to
prevent: two HA Icinga masters send byte-identical notifications, one wins, the
other gets a 500, and Icinga never retries — so the alert is lost.

**Preferred fix** is to wrap in `mapUniquePropertyConflict` (`%w`) so
`errors.As` resolves for every caller, rather than teaching the webhook about a
leaky mapping. Check the 422 mapping still behaves for existing API callers.
