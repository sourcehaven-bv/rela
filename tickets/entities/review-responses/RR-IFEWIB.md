---
id: RR-IFEWIB
type: review-response
title: e2e seeding does 100 sequential createEntity calls
finding: The regression test seeds past the page boundary with 100 sequential HTTP createEntity calls, each writing through entitymanager with automations and validation. The reviewer flagged this as a potential future CI-runtime problem.
severity: nit
reason: 'Measured rather than assumed: the spec runs in 14.4s, and the full 300-test e2e suite is 1.3-1.4m with it included -- so it is not close to dominating. The seed genuinely has to cross the server''s per-page boundary, and the alternatives are worse for this test: `rela dev seed` is a postgres-build raw-store path that bypasses the API the picker reads, and a bulk endpoint does not exist. Revisit if suite runtime becomes a problem; adding indirection now would obscure what the test seeds and why.'
status: wont-fix
---
