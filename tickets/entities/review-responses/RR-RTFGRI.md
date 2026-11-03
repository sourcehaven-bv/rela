---
id: RR-RTFGRI
type: review-response
title: govulncheck stderr hidden when picker succeeds
finding: /tmp/vulncheck-json.err was printed only on the failure branch.
severity: minor
resolution: Printed right after the govulncheck call when non-empty.
status: addressed
---
