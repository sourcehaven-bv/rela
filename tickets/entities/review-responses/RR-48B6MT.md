---
id: RR-48B6MT
type: review-response
title: Grep guard used an inclusion list, silently skipping new evaluation files
finding: 'TestNoDirectRoleLookupInEvaluationPaths iterated four hardcoded filenames. A NEW file added to internal/acl containing a direct policy.Roles[...] lookup would never be scanned — the guard would pass and the ceiling would be silently bypassed on that path. The guard''s own premise (request.go: ''Every evaluation path in this package resolves a role name'') was enforced for only four enumerated files. This is the same forget-shaped hole that applies_to disjointness exists to remove.'
severity: significant
resolution: 'Inverted to an EXEMPTION list: the test now globs *.go, skips _test.go, and skips five explicitly-reasoned exemptions (policy.go, ceiling.go, ceilingcompile.go, declarative.go, request.go — all policy-structure or the clamp itself). A new file fails closed: it must be clean or be added to the exemption list with a reason. Coverage went from 4 files to 13. Added a `scanned < 5` assertion so a broken glob or over-broad exemption list surfaces as a failure rather than a vacuously green run. Verified the guard still fires by injecting a bypass into readquery.go.'
status: addressed
---
