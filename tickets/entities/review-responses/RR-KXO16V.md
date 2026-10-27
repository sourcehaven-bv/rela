---
id: RR-KXO16V
type: review-response
title: Every list call would compute every roll-up
finding: Pickers and autocomplete call the list API often.
severity: minor
resolution: List rows compute roll-ups only when the request opts in with rollups=1 (the SPA list table sets it), like comment counts.
status: addressed
---
