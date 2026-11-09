---
id: RR-R4PWRRM
type: review-response
title: 'Design: source plurals and adoption caveats'
finding: Plurals from _schema are untrusted; Skipped holds relation types only; operator scripts may also write refs; inline HTTP in the CLI needed a reason.
severity: minor
resolution: 'Plan: plurals checked and escaped; empty-type guard covers hidden entity types; adoption caveat documented; the CLI is an operator command, the scheduled path runs as a job.'
status: addressed
---
