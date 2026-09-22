---
id: RR-EL513R
type: review-response
title: 'Fallback collision key was face-blind, and the store trusted a caller-side guarantee'
finding: 'The pre-flight built the mirror as entity.Relation{From: r.To, Type, To: r.From} with a ZERO FromFace, while existing was keyed on r.Key(), which serializes the tail face into the FROM slot. Verified: a stored edge keys as A@draft--blocks--B while the constructed mirror keys as B--blocks--A, so the two can never compare like with like. Unreachable today because the step refuses tailed edges first, which makes it worse rather than better: the BulkMigrator contract said the CALLER guarantees no tail face, so the one component whose correctness depended on that guarantee was also the one that could not see it violated. The same face-blind construction was duplicated in the step''s preview path, so a fix in one place would not have fixed the other.'
severity: critical
resolution: 'The precondition is now ASSERTED by the store rather than assumed: checkSwappable refuses any non-default FromFace outright, and the mirror key carries the tail so it compares like with like. The duplicated logic is gone - both the preview and the write path call one exported store.CheckSwapRelationEndpoints, so they cannot disagree about what is legal, and the dispatcher runs it on the native path too.'
status: addressed
---
