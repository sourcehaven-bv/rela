---
id: RR-J4SUAP
type: review-response
title: Modes and ownership of copied files
finding: secrets mode copied verbatim; audit needs 0600; .rela needs 0700.
severity: minor
resolution: 'Plan updated: .rela 0700, secrets.yaml 0600 with O_EXCL|O_NOFOLLOW, audit files 0600.'
status: addressed
---
