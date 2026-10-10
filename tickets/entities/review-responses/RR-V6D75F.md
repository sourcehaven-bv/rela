---
id: RR-V6D75F
type: review-response
title: CLI exemption not deny-by-default
finding: cliOptions exempted every command but scheduler.
severity: minor
resolution: 'Moot: the exemption is gone; a new surface is ungated only by holding the default handle, and gating is opt-in by FieldGatedEntityManager at wiring.'
status: addressed
---
