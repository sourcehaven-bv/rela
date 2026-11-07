---
id: RR-DTSDBII
type: review-response
title: Subject delete path is face-exact and rename not atomic with starts
finding: Query.Subject was an exact Ref; listing then deleting races a concurrent start; rekey too.
severity: minor
resolution: Query filters by subject id across faces; RekeySubject is idempotent; the start race is carried to TKT-9IT3LN8.
status: addressed
---
