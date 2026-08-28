---
id: RR-CAPSCH
type: review-response
title: "Deps-carried capability grants were silently erased: every scheduled task's capabilities: block was a no-op"
finding: |-
    Engine.execute passes lua.WithCapabilities UNCONDITIONALLY — plain ExecuteFile/ExecuteCode supply an empty lua.Capabilities{}. WithCapabilities assigned straight into r.caps, and options are applied AFTER the runtime is seeded from deps.Capabilities, so the empty option overwrote whatever the deps carried.

    The scheduler (scheduler.go) sets deps.Capabilities from the task's capabilities: block and then calls ExecuteFile. So a task declaring 'capabilities: {http: true, secrets: [upstream_token]}' ran with NO grant and died at 3am with "attempt to index a nil value (global 'http')".

    Confirmed empirically: deps grant alone yields http=table; deps grant plus an explicit zero option yields http=nil.

    This failed CLOSED, so it was a broken feature rather than an exposure. It went unnoticed because the scheduler's only capability test asserted YAML decoding and never that the grant reaches a runtime.
resolution: |
    WithCapabilities now treats a zero-value grant as "no opinion" rather than as a revocation: an empty option leaves a deps-carried grant intact. This is safe because the zero value already means "grant nothing", so an empty option carries no information worth destroying a grant over, and no caller revokes — a runtime is built once per execution from config that either named a capability or did not.

    Pinned by TestDepsCapabilitiesSurviveExecuteFile, plus TestExecuteFileGrantsNothingByDefault (so the fix cannot be satisfied by always granting) and TestExplicitCapabilitiesStillOverrideDeps (the non-empty override still wins). Mutation-verified: reverting to the straight assignment fails the first test with the original symptom.
severity: critical
status: addressed
---
