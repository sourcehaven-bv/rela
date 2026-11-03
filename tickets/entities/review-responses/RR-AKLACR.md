---
id: RR-AKLACR
type: review-response
title: max 0 on a file property became a load error
finding: Only a negative max was an error before; max 0 meant one file.
severity: minor
resolution: 'Restored: only a negative file max is refused; 0 still means one file. Test added.'
status: addressed
---
