---
id: RR-E6ADZK
type: review-response
title: versionServiceFor can leak a typed nil — a hole this refactor opened
finding: 'versionServiceFor returned s.VersionStore() directly. That boxes a concrete *pgstore.VersionStore into store.VersionService, so a nil pointer yields a NON-nil interface — the exact failure the doc comment three lines above promises not to produce. New rather than pre-existing: asserting st.(*pgstore.Store) bounded the reachable implementations to one whose VersionStore() is unconditionally non-nil, so the concrete assertion was doing load-bearing work. Widening discovery removed that bound while the return type stayed concrete. Downstream, versionRecorderFor (appbuild.go:1301) and startDataMigration (datamigration.go:58) both nil-check and would both pass, then panic on first use at write time in production. Same shape applies to storeUserStateFor for a (nil, nil) return.'
severity: critical
resolution: 'Added explicit nil-pointer guards to both resolvers, with a comment explaining WHY the guard became necessary when discovery widened (so it is not deleted later as redundant). Verified the mechanism in an isolated 30-line program: unguarded != nil = true, guarded = false. Added TestCapabilityPresentButHandleNilYieldsUntypedNil covering the capability-present-returns-nil branch, and confirmed it FAILS against the unguarded code.'
status: addressed
---
