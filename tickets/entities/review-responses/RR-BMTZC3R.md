---
id: RR-BMTZC3R
type: review-response
title: Slack in baseline after removals
finding: A lower count only prints a note, so a later PR can refill freed room.
severity: significant
resolution: 'Accepted and mitigated: a PR cannot raise a baseline (-against check), and freed room is only claimable by running -update on develop. Documented.'
status: addressed
---

A lower count only prints a note\, so a later PR can refill freed room.
