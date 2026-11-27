---
id: RR-XCOLGPV
type: review-response
title: Unset of protected property via trigger
finding: Godoc claims file properties are refused but there is no runtime check; unset may slip through.
severity: minor
resolution: Already refused at load for every rule kind; TestSetUnset_LoadErrors now covers relation_created and relation_removed with an external-ref case.
status: addressed
---
