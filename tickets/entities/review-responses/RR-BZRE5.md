---
id: RR-BZRE5
type: review-response
title: SupportsStreaming called per attachment write (O(decorator-depth))
finding: AttachFile calls rooted.SupportsStreaming() on every write. Result is a pure function of the immutable underlying FS — cache at construction time.
severity: minor
resolution: Added streamingSupported bool field on FSStore, populated in New() from cfg.Rooted.SupportsStreaming(). writeAttachment now branches on the cached field instead of walking the decorator chain.
status: addressed
---
