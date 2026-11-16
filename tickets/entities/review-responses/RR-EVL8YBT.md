---
id: RR-EVL8YBT
type: review-response
title: Proposed name rule breaks names valid today
finding: Today a name only has to be dot-free; [a-z][a-z0-9_-]{0,63} would force renames that break audit queries and queued background jobs.
severity: significant
resolution: 'Plan: trigger keys follow today''s rule (non-empty, no dot or colon, unique) and are kept byte for byte; automation-<n> takes the next free n.'
status: addressed
---
