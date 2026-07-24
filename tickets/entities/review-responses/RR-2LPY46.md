---
id: RR-2LPY46
type: review-response
title: SetJWTGate accepts a nil Verifier and panics on first request
finding: SetJWTGate took JWTGateConfig by value and stored it unvalidated. The Verifier field is an interface, so an embedder passing a nil verifier produces a config whose VerifySubject call panics per-request rather than failing at wiring time. The godoc said "Required" but nothing enforced it, violating the project's own "Constructors reject nil required fields" rule (CLAUDE.md). Panic direction is fail-closed (a panicking handler authenticates nobody), so this was a robustness and diagnosability defect, not a bypass.
severity: significant
resolution: SetJWTGate now returns error and rejects a nil Verifier and an empty HeaderName. cmd/rela-server handles the error and exits. The interface-typed-nil caveat (a (*jwtauth.Verifier)(nil) stored in an interface is not == nil and cannot be caught without reflection) is documented on the method, noting that cmd/rela-server checks the concrete pointer before it reaches the interface. Covered by TestSetJWTGate_RejectsIncompleteConfig.
status: addressed
---

Reported by cranky-code-reviewer against `internal/dataentry/app.go:342`.

Fixed in this changeset rather than deferred: it is a small change in the "make
the contract enforceable" category, and the project rule it violates is explicit
in `CLAUDE.md`.
