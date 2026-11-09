---
id: RR-9PZFZCP
type: review-response
title: Every webhook downloads the whole member list
finding: Without a single-member GET the action fetches the full list per event; large orgs pay quadratic cost on bulk changes.
severity: minor
resolution: Noted in docs section 3. Pratique offers no single-member GET, so the list is the only existing endpoint.
status: addressed
---
