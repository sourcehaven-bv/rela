---
id: RR-UEP3A0
type: review-response
title: The wiring test proved the bundle OFFERS the gates, not that the fixture CONSUMES them
finding: 'TestCompileTransitions_AlwaysSuppliesCopyGates asserts CompileTransitions returns non-nil gates. But the reported bug was not that the gates were unavailable — they were available and simply not passed. Someone reverting the fixture to CopyReadGate: nil leaves that test green, so the new test does not cover the half that actually broke.'
severity: minor
resolution: 'Strengthened TestNew_WithDeclarative_WiresBothACLAndDeclarative in appbuildtest, which is the only fixture path reaching entitymanager.New with a policy-backed ACL. It now recovers explicitly and fails with a message naming the regression, rather than dying on an unexplained panic. The two tests are now complementary and the doc comments on each say so: appbuild''s proves the bundle offers the gates, the fixture''s proves the wiring site consumes them. Verified by reverting the fixture''s two lines — both TestNew_WithDeclarative_WiresBothACLAndDeclarative and TestScheduledLuaWriteDeps_ReadsAreACLBound fail; restoring goes green.'
status: addressed
---
