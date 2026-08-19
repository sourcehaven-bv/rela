---
id: RR-EZ0P4S
type: review-response
title: rela acl audit double-reports the ungated-membership condition
finding: internal/cli/acl.go builds services via appbuild.Discover, which runs buildACL -> warnUngatedMembership. Running `rela acl audit` against an open policy therefore emits the slog warning to stderr first, then prints the A1-ungated-membership finding to stdout — two renderings of one problem in one invocation of the audit tool itself.
severity: minor
reason: 'Architect decision (2026-08-19): no follow-up ticket. The stutter is cosmetic, the two outputs agree by construction (shared predicate), and the boot warning firing on EVERY policy load — including the audit command''s own load — is the accepted RR-62ZH2M behaviour: one unconditional call site in buildACL with no command-aware special cases. Threading a suppression option through Discover/buildACL would add wiring surface solely to hide a consistent warning from the one command whose users are already looking at findings.'
status: wont-fix
---
