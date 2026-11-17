---
id: RR-13MJ1TP
type: review-response
title: Partial accept failure
finding: Accept issues a PATCH and several relation creates; any can fail. Report per item; no silent partial success.
severity: minor
resolution: LLMProposalDialog applies each item separately and lists every failure by item (stale property, save error, link error); it never reports success when one failed.
status: addressed
---
