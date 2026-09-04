---
id: RR-WQ3DQS
type: review-response
title: 'Seeder is a fourth raw-store exception: attribution, sweep interaction and CLAUDE.md list'
finding: Root CLAUDE.md enumerates three raw-store exceptions, each with operator-shell trust, explicit audit and store.WithAttribution. The plan names audit but not attribution, and does not decide what the version sweep does with 20k bulk-inserted rows. kong supports hidden commands but there is no precedent; hidden is obscurity, not a boundary.
severity: minor
resolution: Seeder writes with store.WithAttribution(system:perf-seed) and an audit record; the sweep capturing one version per seeded row is accepted as realistic load and documented; CLAUDE.md's exception list becomes four entries. The command is not hidden; it lives under `rela dev seed` with help text stating it is a raw-store developer tool.
status: addressed
---
