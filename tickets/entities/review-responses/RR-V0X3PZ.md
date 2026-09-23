---
id: RR-V0X3PZ
type: review-response
title: Test gaps for face pins and refusal paths
finding: No storetest for outbound or chained named-face tails; no e2e unsupported path; budget uses outgoing only and naive per-candidate cost is below the counting wrapper.
severity: minor
resolution: Added storetest outbound_named_face_tailed_edge_does_not_match and chain_second_hop_named_face_tailed_edge_does_not_match (mem/fs/sqlite/pg), the acl candidate-face test and the 422 e2e test. The budget's naive-backend limit is recorded on RR-UAL3GB; the pg EXPLAIN covers the SQL shape.
status: addressed
---
