---
id: RR-4RJZH2G
type: review-response
title: 'Nits: type name, runner contract, nil constructor args'
finding: InMemory is a vague name; ApplyWithMetamodel changed its encode-error contract; constructors do not reject nil fs.
severity: nit
resolution: Renamed to Migrated; restored the old encode-error contract. Nil fs is the existing NewFSLoader pattern, left as is.
status: addressed
---
