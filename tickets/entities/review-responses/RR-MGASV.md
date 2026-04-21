---
id: RR-MGASV
type: review-response
title: resolve doc overstates the invariant — symlink escapes possible
finding: resolve() is a string-level validator only; it rejects traversal in keys but the OS follows symlinks inside root. The doc comment said 'single path-validation barrier' without qualification, which misleads future readers and CodeQL-triage reviewers.
severity: significant
resolution: 'Tightened doc comments in rooted.go: described resolve as the ''string-level path-validation barrier'' and added a Security section explicitly stating that symlinks inside the root are followed and that is out of scope. Threat model documented.'
status: addressed
---
