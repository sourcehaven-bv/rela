---
id: RR-C5TPB6
type: review-response
title: seedRowOf read another face when a named face was missing
finding: seedRowOf fell back to any face when ID@face named a face with no row, so edit("POL-4@review") edited @draft and hidden{id="POL-4@review"} passed trivially.
severity: significant
resolution: An ID@face address now reads that face only and returns ErrNotFound otherwise; only a bare id scans the family. TestSeedRowOf gained a missing-face case.
status: addressed
---
