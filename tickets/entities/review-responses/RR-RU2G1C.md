---
id: RR-RU2G1C
type: review-response
title: RelationKey.String called not a wire format but Key() is the pg cursor
finding: Relation.Key() now delegates to RelationKey.String(), whose doc said it is not a wire format. pgstore encodes Key() as the relation paging cursor and parses it back, so a log-text tweak would break cursors silently.
severity: significant
resolution: 'String() doc now states the format is stable: it is what Relation.Key returns and what the pg relation cursor encodes and parses.'
status: addressed
---
