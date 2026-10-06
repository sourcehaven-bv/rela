---
id: RR-HNCM4I
type: review-response
title: Edge budget test counts its own listing and has one peer type
finding: The pin includes the test's ListEntities, and all peers are tickets, so a per-type cost is not pinned.
severity: minor
resolution: The pin comment names the setup listing. The test now adds feature peers, so two types are gated; the pin stays 4.
status: addressed
---
