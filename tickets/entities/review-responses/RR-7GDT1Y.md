---
id: RR-7GDT1Y
type: review-response
title: get_relations peer-visibility semantics differ from today's from/to filters — pin the interaction
finding: 'rela.get_relations(opts) accepts from/type/to filters and streams matching relations with no peer gating. After FilterRelations, a relation whose FROM the caller can see but whose TO is hidden disappears entirely. That is correct per the both-endpoints rule, but it changes the meaning of an explicit opts.from query: asking ''relations from X'' can now return fewer rows than the graph holds, with no signal. Pin it with a test (explicit from-filter + hidden TO → row absent) and document that get_relations is peer-gated, so script authors don''t read an empty result as ''no such edges''.'
severity: minor
resolution: AC4 extended to cover the explicit opts.from case (visible FROM + hidden TO → row absent), and the docs will state that get_relations is peer-gated so an empty result is not read as 'no such edges'.
status: addressed
---
