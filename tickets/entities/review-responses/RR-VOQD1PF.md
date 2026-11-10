---
id: RR-VOQD1PF
type: review-response
title: 'Security review: second search_path spelling in base DSN'
finding: A base DSN with SEARCH_PATH or options=-c search_path could override the pinned path on some connections.
severity: minor
resolution: 'dsnForSchema refuses any other spelling of search_path and an options value that sets it. Test: TestDSNForSchema_RefusesOtherSearchPathSpellings.'
status: addressed
---
