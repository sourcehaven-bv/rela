---
id: RR-CAJCU
type: review-response
title: Method validation says 'must be valid HTTP method' but doesn't define valid set
finding: 'Plan says ''must be valid HTTP method string'' for the method field. Go''s net/http accepts any string as a method. Should we validate against a known set (GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS) or let anything through? Given the use case is calling APIs, restricting to the standard set seems right but should be explicit. Also: should the method be case-insensitive (auto-uppercase)?'
severity: nit
resolution: Auto-uppercase method, accept any non-empty string
status: addressed
---
