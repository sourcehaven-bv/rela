---
id: RR-3T6P1ZF
type: review-response
title: Refused rename leaves no audit record
finding: 'pr-guidance: a rename or conversion the rename guard refuses was not written to the audit log, unlike a refused field write.'
severity: minor
resolution: recordRenameRefusal writes a denied-write record for the subject when a rename or conversion is refused with ErrRenameBlocked; a dry run writes none. Pinned in TestRenameGuard and TestRenameGuard_Conversion.
status: addressed
---
