---
id: RR-K1CYFH5
type: review-response
title: Collab relay enabled headless with build check disabled
finding: EmbeddedBuildID is empty in headless, which the relay treats as development mode, and no client exists.
severity: minor
resolution: buildApp skips EnableCollab in a headless build; the route answers 404.
status: addressed
---
