---
id: RR-JVSQ9
type: review-response
title: 'GetGitUser: shared 5s timeout across two git config calls'
finding: In internal/automation/template.go:44-53 both git config user.name and git config user.email share a single 5s deadline. If the first call takes 4.9s (unlikely but possible on a cold NFS home), the second gets 100ms. git config is essentially instant in practice, so this is a nit; note for future edit.
severity: minor
reason: Minor nit; git config is essentially instant in practice and sharing a 5s budget is fine. Two calls in sequence take <1ms normally. Making them independent would double the boilerplate for no real benefit.
status: wont-fix
---
