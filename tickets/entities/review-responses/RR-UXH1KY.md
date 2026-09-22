---
id: RR-UXH1KY
type: review-response
title: commandHandler.files had no nil guard, violating the constructors-reject-nil rule
finding: Root CLAUDE.md requires constructors to reject nil required fields, but commandHandler is assembled by bare struct literal at two sites (app.go and the test helper) with no New* function. files was dereferenced unconditionally in three places, one of which (release) runs mid-SSE after the 200 and after the script already ran — the worst place to discover a wiring bug. This commit added the field and had to remember to patch both existing sites, which is the accretion pattern the rule exists to catch.
severity: minor
resolution: 'Made a nil *commandFileStore usable and inert: mint returns errNoCommandFileStore, lookup finds nothing, release is a no-op. A third construction site that forgets the field now degrades to ''no download buttons'' instead of panicking mid-stream. Deliberately NOT the shape used for aclImpl, where nil means deny — an authorization guard must fail closed, while a missing download table can only fail to offer a capability. Documented that distinction at the declaration.'
status: addressed
---
