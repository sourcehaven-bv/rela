---
id: RR-MYCY4Q
type: review-response
title: No index on version_tags name
finding: The query filters version_tags by name and the primary key is (vseq name) so it scans the tag table.
severity: minor
reason: The scan is proportional to the number of tags which only grow by explicit tagging. An index needs a PostgreSQL migration and a SQLite schema version bump while other branches add migrations. Tracked as a backlog ticket.
status: deferred
---
