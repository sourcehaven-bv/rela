---
id: RR-E80D4
type: review-response
title: NewRootedFS accepted nil fs; first method call would panic
finding: Constructor validated root but not fs. Nil FS passed in would nil-deref on first use. For a type meant to enforce wiring discipline, failing loud at construction is the whole point.
severity: significant
resolution: 'Added nil-check in NewRootedFS returning ''storage: RootedFS fs must not be nil''. Test TestNewRootedFS_RejectsNilFS added.'
status: addressed
---
