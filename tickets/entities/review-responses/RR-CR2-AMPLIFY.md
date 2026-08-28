---
id: RR-CR2-AMPLIFY
type: review-response
title: openCustomEntry buffered 4 MiB before rejecting on size, on an unauthenticated route
finding: The size check ran AFTER io.ReadAll(io.LimitReader(f, max+1)), so one oversize file in custom/
  cost a 4 MiB disk read and allocation per request. The route is deliberately outside the JWT gate, so
  this is reachable by an unauthenticated client; the reviewer measured 50 requests -> 200 MB read plus
  50 WARN lines. The Stat call two lines earlier already had info.Size() available for free.
severity: significant
status: addressed
resolution: Moved the check to info.Size() > maxCustomFileBytes before the read, with a comment stating
  why order matters on an unauthenticated route. The ReadAll+LimitReader remains as a belt-and-braces
  guard against a file growing between stat and read. Also resolves the oversize half of RR-CR2-DIVERGE.
---

Raised by `/code-review` of the TKT-IWMETE implementation.
