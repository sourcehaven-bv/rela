---
id: RR-R375DS
type: review-response
title: config 0-floor TODO had no enforcement pressure
finding: A 0-floor override with a TODO comment can never fail and would rot indefinitely; internal/config is real production code (validateName guards attacker-controllable file names) with zero tests.
severity: minor
resolution: 'Took the reviewer''s option (b): wrote the loader test in this PR (round-trip, os.IsNotExist contract, 10-case unsafe-name table incl. NUL/traversal/drive-letter, Subscribe validation) — 92.6% coverage; floor set to 87.'
status: addressed
---
