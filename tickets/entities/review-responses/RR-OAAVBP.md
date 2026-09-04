---
id: RR-OAAVBP
type: review-response
title: Step property validation checks only one type when find and create types differ
finding: 'validate_webhooks.go:195-199 initializes stepType to hook.CreateType() then UNCONDITIONALLY overwrites it with hook.Find.Type whenever Find != nil. Only one type is ever checked, but at runtime the steps apply to whichever entity the pipeline ended up with — the FOUND one or the CREATED one. Those are genuinely different types when a hook finds alpha and creates beta (a shape CreateType() at webhook.go:137-148 explicitly permits), so a single-type check cannot be correct in either direction. Verified with disjoint types alpha.a_only / beta.b_only: (1) FALSE POSITIVE — ''set b_only'', correct for the CREATED type, is REFUSED at load with an error naming type alpha, so a legitimate hook prevents the server starting and the message actively misdirects; (2) FALSE NEGATIVE — ''set a_only'', invalid for the created type, PASSES validation, and on the create path applySteps then patches a beta entity with a property beta does not define. The second is exactly the ''config passes validation then misbehaves at request time'' outcome the load-error contract exists to prevent. Contrast that proves the diagnosis: create_if_missing.properties naming b_only validates cleanly because validateWebhookShape:99-104 correctly checks it against CreateType() — the create-properties path gets the type right, the steps path does not. Fix: when Find != nil && CreateIfMissing != nil and the types differ, validate each then.set property against BOTH types and require it on both (a step runs against either, so it must be valid for either), or refuse a find/create type mismatch at load with an explicit message. The silent single-type check is the one option that cannot be made correct. Root cause of the gap: TestWebhookRoutes_ThreeWorkflows only exercises hooks where find and create types AGREE.'
severity: significant
resolution: 'validateWebhookSteps now checks each then.set property against EVERY type a step could run against, via a new webhookStepTypes(hook) helper returning the deduplicated {found type, created type}. A property must exist on all of them, since the pipeline cannot know at load time which branch a delivery takes. Covered by TestValidateWebhooks_StepTypesCoverFindAndCreate, a table over the four cases with a deliberately mismatched find-alpha/create-beta hook: shared property accepted; found-only rejected; created-only rejected; neither rejected. That table is exactly the coverage gap that let the bug ship — the pre-existing three-workflow test only used hooks where the two types agree.'
status: addressed
---

Both directions are wrong, and they fail in opposite ways — a false positive
that refuses a legitimate config at load, and a false negative that lets a
broken one through to runtime.

stepType := hook.CreateType() if hook.Find != nil { stepType = hook.Find.Type //
<- unconditional overwrite }

Verified with disjoint types `alpha.a_only` / `beta.b_only` on a
find-`alpha`/create-`beta` hook:

set b_only  (valid on CREATED beta)  => ERROR: unknown property "b_only" on type
"alpha" set a_only  (valid on FOUND alpha)   => accepted, then patches beta at
runtime

The contrast that confirms the diagnosis: `create_if_missing.properties` naming
`b_only` validates cleanly, because `validateWebhookShape:99-104` checks it
against `CreateType()`. The create-properties path gets the type right; the
steps path does not.

The coverage gap that let it ship: `TestWebhookRoutes_ThreeWorkflows` only
exercises hooks where the two types agree.
