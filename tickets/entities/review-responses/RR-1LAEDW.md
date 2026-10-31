---
id: RR-1LAEDW
type: review-response
title: Grouped-list busy cost is understated
finding: 'A grouped list loads up to max_rows, default 500 and at most 2000 (validate_groupby.go:36-37), in pages of 100. That is up to 20 busy reads per load, and each live frame refetches the whole set with 20 more. On the file backend each read is one directory read per anchor, so 2000 directory reads per refresh. The safety replay also re-serves the 100-row live path with a busy read for every open grouped tab, once per safety period. The risk section calls this ''the same per-page cost as the board'' without numbers. Fix: state the worst case and either accept it explicitly or note a follow-up (for example, send busy only on the live path''s first 100 rows).'
severity: minor
resolution: ADDRESSED risk section states the worst case (20 busy reads per load and per live frame, about 2000 directory reads per refresh on fsstore, plus one safety-replay read per grouped tab) and accepts it, with a follow-up idea to send busy only on the first 100 rows
status: addressed
---
