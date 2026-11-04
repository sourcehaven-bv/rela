---
id: RR-3DQU1D2
type: review-response
title: RequestURI can be absolute-form
finding: parseLivePath rejects an absolute-form RequestURI.
severity: nit
resolution: recordServed builds the path from URL.EscapedPath and RawQuery.
status: addressed
---
