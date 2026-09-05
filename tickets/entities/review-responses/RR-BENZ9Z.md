---
id: RR-BENZ9Z
type: review-response
title: copyOnSuccessWire encodes a mode>world>face precedence the validator forbids
finding: internal/dataentry/affordances.go copyOnSuccessWire's ordered switch resolves the world+face case the validator rejects, a silent divergence waiting for the validator to be relaxed. Mirror the mutual exclusion explicitly.
severity: minor
resolution: copyOnSuccessWire's world and face arms each require the other field empty, mirroring the loader's mutual exclusion; the impossible both-set shape falls to written, and the comment says why.
status: addressed
---
