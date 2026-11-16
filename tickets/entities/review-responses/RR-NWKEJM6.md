---
id: RR-NWKEJM6
type: review-response
title: Migration index build and the alias shadowing need comments
finding: The GiST build holds the migration lock (fine while every value is empty) and the anti-join relies on the inner alias e shadowing the outer one. Both are easy to break later. The security review also noted the pinned search_path depended on the DSN.
severity: nit
resolution: Comments added in 0047_search_title.sql and worldPrimeSQL. The pinned search_path is gone (TG_TABLE_SCHEMA).
status: addressed
---
