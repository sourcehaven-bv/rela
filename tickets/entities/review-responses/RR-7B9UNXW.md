---
id: RR-7B9UNXW
type: review-response
title: Retry responses not drained; Bearer prefix doubled; misc
finding: 429/503 bodies were not drained before retry; a token starting with Bearer was sent as Bearer Bearer; flag checks ran after the remote was built; Nil comment form; uint64 sum could wrap.
severity: nit
resolution: Drained up to 64 KiB; Bearer prefix stripped case-insensitively; flags checked first; Nil comment on the field; the small-span test is overflow safe.
status: addressed
---
