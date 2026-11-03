---
id: RR-HYKXJM
type: review-response
title: No test pins the SQL to the Go lineage reads
finding: The SQL repeats the lineage rules of ListVersions and VersionByTag. No test compared the two on rename chains or on random histories.
severity: significant
resolution: Every storetest TaggedEntities call now checks each row against ListVersions and VersionByTag. Added a rename chain test and a rename back test and a randomized model test over 12 seeds of creates and updates and renames and deletes and tags on both database backends.
status: addressed
---
