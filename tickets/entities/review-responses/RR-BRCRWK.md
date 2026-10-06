---
id: RR-BRCRWK
type: review-response
title: Copy takes the lock before validating the id
finding: An invalid id produced a lock-key error that masked the planning error.
severity: minor
resolution: lockCopyTarget takes no lock for a malformed target or id; planning reports it.
status: addressed
---
