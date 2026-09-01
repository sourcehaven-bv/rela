---
id: RR-O2HRM3
type: review-response
title: rawStateStore's structural coupling to state.KV was undocumented
finding: rawStateStore is a structural duplicate of state.KV's method set, declared separately because pgstore may not import internal/state (arch-lint). Nothing said what happens when state.KV gains a method — a reader could reasonably assume the two drift silently and that this is a latent bug.
severity: minor
resolution: 'Documented that the coupling is self-announcing: if state.KV gains a method, state.NewValidatedKV stops accepting a rawStateStore and the build breaks at the call site with a clear message. The enforcement is the compiler, not the comment; the comment just says so.'
status: addressed
---
