---
id: RR-5DUDWNZ
type: review-response
title: Memory and DoS from multipart posts
finding: Whole-body in-memory parse; bytes read before the read/post checks; rate bucket counts posts not bytes.
severity: significant
resolution: Total cap 16 MiB per message; Limiter slot taken before reading the body; multipart read part by part with the message part required first; read/post-role/rate checks before file parts; files spool to disk; a lower rate bucket for posts with files.
status: addressed
---
