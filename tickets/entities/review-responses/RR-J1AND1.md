---
id: RR-J1AND1
type: review-response
title: Recovery test could pass with the store in a bad state
finding: The test collapsed edges per type, accepted any error and did not check the state between the failed run and the re-run.
severity: significant
resolution: Tests assert the exact edge set with properties via assertEdges, use a sentinel error, and check that the source row and the uncarried edge survive the failed run.
status: addressed
---
