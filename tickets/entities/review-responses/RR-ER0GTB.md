---
id: RR-ER0GTB
type: review-response
title: Watcher still admits undeclared types; the new entityFileKey invariant comment is false
finding: 'internal/store/fsstore/layout.go:107 claims every indexed type is in paths, so the ''+s'' fallback is unreachable. internal/store/fsstore/watcher.go reconcileEntityPath/parseEntityFromPath (~line 387) takes the type from frontmatter and never checks that the directory maps to a declared type. An externally written entities/riskregister/RSK-1.md with type: risk is therefore indexed while risk is undeclared, and so is a file in requirements/ whose frontmatter says type: solution. entityFileKey then resolves the wrong directory and the entity becomes unreadable or undeletable. This is the gate this ticket adds, entered from the side. Fix: in reconcileEntityPath, require entityIdentityFromPath to succeed and its directory type to equal the frontmatter type (skip with slog.Warn otherwise, as the open scan does), then make the comment true. Add a watcher test.'
severity: significant
resolution: reconcileEntityPath indexes a file only when its directory maps to a declared type matching the frontmatter type and the frontmatter id matches the filename stem; the entityFileKey comment is accurate.
status: addressed
---
