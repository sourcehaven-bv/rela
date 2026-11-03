---
id: RR-RFOER7
type: review-response
title: The summary handler text misdescribes the inbox route
finding: Step 2 says query parameters are ignored "as on the inbox route", but the inbox route parses limit, cursor and kinds (inbox_handler.go:45-63). Say that the summary ignores its query string, while the live path refuses any query on the summary (AC 5), and test that GET /_inbox/summary?limit=abc answers 200.
severity: nit
resolution: Step 2 now says the summary ignores its query string while the live path refuses any query on it, and AC 1 tests that ?limit=abc answers 200.
status: addressed
---
