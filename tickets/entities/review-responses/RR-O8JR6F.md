---
id: RR-O8JR6F
type: review-response
title: Grant check refuses every face-anchored set and every faced create
finding: 'Plan Approach 4/5 (plan:302-304, 329-331) uses GrantsVerbOnState only when a set output names a face, else PrincipalGrants -> grantsVerb, which skips every type@face entry (internal/acl/policy.go:568-598). On a face-anchored channel type a set never names a face: setFace refuses one (internal/realtimecfg/outputs.go:171) and the runner writes key.Face (internal/realtime/outputs.go outputTarget). A bare update grant on a faced type is refused at policy load (policy.go:1546), so for a faced attach_to type no acl.yaml can pass the check: a false load refusal. The create branch has the same gap: a create output on a faced type carries Face (outputs.go:183-187) but the plan checks PrincipalGrants(create, type) with no face. Plan change: PrincipalGrants takes a face; for a set on a face-anchored type require GrantsVerbOnState for every face the type declares (the channel may anchor any), for a family-anchored set use the named face, for a create use o.Face. Add AC 6 rows for a face-anchored faced type and a faced create, and cover both in TestRunAsGrantCheck_AgreesWithRequest.'
severity: significant
resolution: JobGrants.Verb takes a face via GrantsVerbOnState; face-anchored sets need update on every declared face, family sets the named face, creates the output face. AC 6 and AC 7 rows added.
status: addressed
---
