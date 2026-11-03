---
id: RR-3Q9DK5
type: review-response
title: AC 14 affordance test includes cases Head cannot express
finding: 'AC 14 runs "the AC 11 cases" through Head and Typing. But in several of those cases Head itself fails or cannot see the reason: the read denial and the unknown many-name (Head answers the same 403/404, so there is no _actions), the 429 rate limit (affordances never predict rate, by the existing contract), and the LogOnly 422 (the HTTP head is never LogOnly). Plan change: AC 14 covers typing off, no post role of any log type, no principal, closed or archived, and the allowed case. State explicitly that 429 is outside the affordance contract, as for _actions.post.'
severity: minor
resolution: 'Plan updated: AC 16 covers typing off, no log post role, no principal, closed, archived and allowed; 429 is stated as outside the affordance contract, as for _actions.post.'
status: addressed
---
