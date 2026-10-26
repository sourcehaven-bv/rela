---
id: RR-K8PE26
type: review-response
title: ValidateRunAs accepts plain user names
finding: 'principal.ValidateRunAs returns nil for a name with no prefix (principal.go:707-726). The plan''s wording ("Checked with principal.ValidateRunAs ... A plain name is refused") reads as if the call refuses plain names. Plan change: state that realtimecfg adds its own check that the name starts with system: or integration:, and keep the AC 2 row.'
severity: nit
resolution: 'Deferred to 5b: run_as moves to 5b, along with the realtimecfg system:/integration: prefix check, because ValidateRunAs accepts plain names.'
status: addressed
---
