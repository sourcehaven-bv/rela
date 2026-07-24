---
id: RR-JGVD30
type: review-response
title: Store-load faults are swallowed as clean misses — deliberate, but the godoc doesn't say so
finding: Get/AllowAllReader/redactStepTitle collapse every store error (incl. context cancellation, transient I/O) into the indistinguishable miss — required by the RR-NGMI oracle-free contract and faithful to the former App.getVisible behavior, but the Get godoc ('Only a gate failure is an error') doesn't state that store faults are therefore operationally invisible. Document the asymmetry so an operator debugging phantom 404s knows where to look.
severity: minor
resolution: 'Reader.Get godoc now documents the asymmetry: store-load faults are deliberately swallowed into the oracle-free miss, so a backend outage reads as 404s — operators debugging phantom misses should check store health, not the gate.'
status: addressed
---
