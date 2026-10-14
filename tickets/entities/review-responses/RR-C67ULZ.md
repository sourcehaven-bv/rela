---
id: RR-C67ULZ
type: review-response
title: Gate re-parses a raw map in each consumer
finding: gatedListQuery wraps the raw map and applyV1Filters, the pushdown and the relation pass each re-parse it with their own code, and the relation-key classifier is built from separate Meta/Cfg loads, so a config reload or a parser change can reopen the bypass. Use one parser and carry the gate's classifier in the gated query.
severity: significant
resolution: One parser, parseFilterKey, serves the gate, applyV1Filters, the relation pass and the pushdown. gatedListQuery carries the gate's isRelationKey and every consumer routes keys with it. TestListFieldGate_SharedKeyParser pins both.
status: addressed
---
