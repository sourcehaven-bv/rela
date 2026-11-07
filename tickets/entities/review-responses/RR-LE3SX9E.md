---
id: RR-LE3SX9E
type: review-response
title: Relation ETag ignores formatting-only body edits
finding: The relation ETag is the canonical relation digest; its body is FormatMarkdown-normalized so a reflow-only edit of a relation body on a raw-storing backend keeps the ETag.
severity: minor
resolution: 'Fixed after code review: the relation ETag now goes through the same viewETag helper as the entity ETag (view digest plus the raw served body) so a formatting-only relation body edit moves it too.'
reason: Pre-existing behaviour since the relation read endpoint shipped; the endpoint has no If-Match write path. Folding the raw body would make the relation ETag differ from the plain view digest. Left as is and recorded for a follow-up if relation conditional writes arrive.
status: addressed
---
