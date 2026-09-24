---
id: RR-6TZPNA
type: review-response
title: Postgres endpoint join matches any face and any edge tail
finding: buildPredicateSQL/nestedPredicateSQL joined entities on id only and never filtered r.from_face, so a draft face or draft-tailed edge could satisfy a traversal in the default world (draft content disclosure). The Go backends read only the default endpoint face, so backends also disagreed.
severity: significant
resolution: 'Endpoint-match hops are pinned to the default state on every backend: pgstore defaultStateCond (r.from_face = '''' AND _ep.face = ''''), graphquerynaive FromFace=&zero. Contract documented on RelationPredicate.EndpointMatch. Storetest cases named_face_of_the_endpoint_does_not_match and named_face_tailed_edge_does_not_match pass on fs, mem, sqlite, pg.'
status: addressed
---
