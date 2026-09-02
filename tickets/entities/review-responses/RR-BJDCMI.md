---
id: RR-BJDCMI
type: review-response
title: Widening VersionStore() to an interface silently broke the typed-nil guard
finding: TKT-L3FNEN changed pgstore.Store.VersionStore() from returning a concrete *VersionStore to returning store.VersionService. The `vs == nil` check in versionServiceFor was carried across unchanged, but it only works on a POINTER. Once the callee returns an interface type, a backend doing `var p *MyStore; return p` boxes a typed nil, the interface compares non-nil, every downstream nil-check passes, and the panic lands at first write in production. The commit message claimed to strengthen this guard; it disabled it, in the same change that made the hazard reachable by more implementations.
severity: critical
resolution: 'Added internal/appbuild/capability_postgres.go with nonNilCapability[T comparable] plus a reflect-based wrapsNil, and wired it into both versionServiceFor and storeUserStateFor. One shared helper rather than a per-resolver check, deliberately: three hand-rolled guards were what let the regression through, because ''a guard exists'' was true at all three sites while ''the guard works'' was true at one. Mutation-verified: removing the helper makes TestNeutralProviderReturningNilYieldsUntypedNil/typed_nil_pointer and TestCapabilityPresentButHandleNilYieldsUntypedNil/version_service both fail.'
status: addressed
---
