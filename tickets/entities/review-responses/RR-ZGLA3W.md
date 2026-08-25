---
id: RR-ZGLA3W
type: review-response
title: The negative nil test could not reach the branch that actually regressed
finding: TestResolversReturnUntypedNilWithoutCapability was described as the load-bearing negative test, but it only exercised a store with NO capability — which hits a `return nil` literal and can never catch a typed nil. The dangerous branch is capability-present-returns-nil-handle, for which no fake existed. fakeUserStateStore.UserState returned (nil, nil) as an untyped nil interface, exercising nothing.
severity: significant
resolution: 'Added fakeNilVersionStore (returns a nil *pgstore.VersionStore) and fakeNilUserState (returns (nil, nil)), plus TestCapabilityPresentButHandleNilYieldsUntypedNil. Verified non-vacuous: it fails against the unguarded resolver with ''versionServiceFor = (*pgstore.VersionStore)(nil), want untyped nil''.'
status: addressed
---
