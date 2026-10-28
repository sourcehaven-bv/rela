---
id: RR-LMAO3K
type: review-response
title: The schema unguarded_path reason was wrong
finding: The reason called schema a CLI tool, but dataentry and mcp also call it with a reader they supply.
severity: minor
resolution: 'The reason now says schema is a library over the caller''s reader: server callers pass the gated reader, the CLI passes schema.Ungated under operator trust.'
status: addressed
---
