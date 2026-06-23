---
id: RR-9RHEWN
type: review-response
title: Relation manifest key (--) collided with path key (/) and was ambiguous
finding: 'manifestKey rendered a relation as from--type--to, but the PUT path parses from/type/to (slash). Asymmetric encoding the client had to translate. Worse: validIDSegment permits ''-'', so ''--'' is a legal substring of any segment — a relation type containing ''--'' (or an entity id) made the manifest key AMBIGUOUS and un-reversible, a silent sync-drift correctness bug.'
severity: significant
resolution: 'Unified the encoding: manifestKey now renders a relation as from/type/to (slash-joined), the SAME form the PUT path uses, so a manifest entry''s id is directly usable as the path tail. Slashes can''t appear in a segment (validIDSegment rejects them), so the slash join is unambiguous — unlike ''--''. Test TestSync_ManifestSerialization asserts the slash form.'
status: addressed
---
