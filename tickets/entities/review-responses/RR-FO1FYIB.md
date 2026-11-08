---
id: RR-FO1FYIB
type: review-response
title: Oversized webhook bodies are truncated
finding: LimitReader truncated a body over 64 KiB and verification failed as 401.
severity: nit
resolution: Uses http.MaxBytesReader and answers 413. Test row added.
status: addressed
---
