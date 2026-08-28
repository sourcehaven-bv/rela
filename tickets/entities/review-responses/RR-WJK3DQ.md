---
id: RR-WJK3DQ
type: review-response
title: stateKVFor was left unwidened while the test implied parity with the other three
finding: The commit message claimed four assertion sites were replaced; three were. stateKVFor still calls pgstore.StateStoreFor(st), which type-asserts *pgstore.Store internally (statekv.go:72). A second backend would therefore get version sweeps, user state and derived schema by interface, then silently fall back to node-local FSKV for state — partial capability adoption, which is worse than none because it degrades quietly at runtime rather than failing loudly at wiring. TestResolversReturnUntypedNilWithoutCapability tested stateKVFor alongside the widened three, implying a parity that does not exist.
severity: significant
resolution: 'Documented the exclusion at the call site with the runtime consequence and a TKT-L3FNEN pointer; annotated the test so stateKVFor is understood as covering its nil contract only, not interface parity; and expanded TKT-L3FNEN with a dedicated section stating this must close BEFORE a second backend ships, not merely before the work is ''complete''. Not widened here: StateStoreFor is a package function by design (pgstore''s plimsoll line forbids growing its method set), so changing it is the same promote-the-types problem TKT-L3FNEN owns.'
status: addressed
---
