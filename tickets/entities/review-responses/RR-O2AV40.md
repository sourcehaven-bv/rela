---
id: RR-O2AV40
type: review-response
title: 'C4: ExpectType is not a one-line swap; equalsType is asymmetric and AnyType rejects everything'
finding: 'Plan said replace the hardcoded assertion at compile.go:109 with the configured type. But Type comparison methods are unexported and equalsType is asymmetric: AnyType.equalsType matches only literal AnyType so ExpectType(AnyType) would reject everything (opposite of the obvious reading); RecordType demands exact field-set equality; DateType ignores layout.'
severity: critical
resolution: ExpectType restricted to scalar types with its argument validated at Compile entry - rejects RecordType ListType AnyType - mirroring DeclareFunc env.go:168-173. A test pins that ExpectType(DateType) accepts a DateTypeWithLayout result since that asymmetry is load-bearing for rrule_next.
status: addressed
---
