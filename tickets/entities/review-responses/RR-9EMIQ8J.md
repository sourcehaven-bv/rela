---
id: RR-9EMIQ8J
type: review-response
title: pg test Second pool can land on the wrong schema
finding: Second used cfg.ConnString(), which keeps a search_path from the DSN.
severity: minor
resolution: Second opens a pool through openPool, which sets search_path on the parsed config.
status: addressed
---
