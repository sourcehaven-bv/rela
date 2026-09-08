---
id: copy-read-gates-required-under-policy
type: automated-measure
title: Copy read gates are mandatory under a compiled ACL policy
description: entitymanager.New refuses a Deps whose ACL is *acl.Declarative while CopyReadGate or CopyVisibility is nil, so forgotten copy-gate wiring is a construction failure rather than a silent ungated read. TestNew_CopyGatesRequiredUnderPolicy pins both tiers (refused under a compiled policy, still permitted under NopACL/ReadOnlyACL so the CLI case cannot regress); TestCompileTransitions_AlwaysSuppliesCopyGates pins the wiring layer, asserting the shared TransitionWiring bundle always populates both so the constructor check stays a backstop rather than the only line.
kind: test
location: internal/entitymanager/copy_gate_required_test.go + internal/appbuild/copygatewiring_test.go
status: active
---

Two layers, because the defect had two halves.

**The constructor check** (`internal/entitymanager/manager.go`,
`requireCopyGates`) makes forgotten wiring fail fast.
`TestNew_CopyGatesRequiredUnderPolicy` exercises both-nil, half-nil, and
explicit opt-out under a compiled policy, and — importantly — asserts that
`NopACL` / `ReadOnlyACL` still build *without* the gates. That second half is
what stops the guard from being tightened into something that breaks the
no-policy CLI posture.

**The wiring invariant** (`internal/appbuild/copygatewiring_test.go`) asserts
`CompileTransitions` always populates `ReadGate` and `Visibility`, in both the
policy and no-policy tiers. This is what keeps the constructor check a backstop
that never fires in practice: every wiring site takes both gates from the one
bundle, so a site cannot omit one by hand-copying an incomplete literal — which
is precisely how the `appbuildtest` fixture came to sit in the unsafe state.

Verified non-vacuous: reverting only the fixture's two gate lines fails the
`appbuild` suite with the new error; restoring them goes green.
