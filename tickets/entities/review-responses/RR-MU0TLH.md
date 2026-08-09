---
id: RR-MU0TLH
type: review-response
title: FieldWriteGate is inert in production but its godoc reads as if it were live
finding: internal/appbuild/appbuild.go is the sole production wiring site and hardcodes AllowAllFieldGate{}, so the gate is unreachable outside tests. This WAS honestly disclosed (TKT-0XL8MF filed, IMPL-T8S8LU states it plainly) — but the Deps.FieldGate godoc said request-scoped surfaces 'wire a policy-backed implementation' in the PRESENT tense, which is aspirational. An inert gate that looks live is how people come to build on protection that was never real. The disclosure belongs where a reader will encounter it, not only in a ticket.
severity: minor
resolution: 'Added an explicit STATUS paragraph to the FieldWriteGate godoc: ''no production surface wires a real implementation yet... Do not assume a write reaching PatchEntity has been field-gated'', pointing at TKT-0XL8MF. Corrected the Deps.FieldGate godoc from the aspirational present tense to ''where a policy-backed implementation belongs — none is wired yet''. Introducing a required-but-inert authz seam remains the right call (it forces the wiring decision at every future call site), but it now says so.'
status: addressed
---
