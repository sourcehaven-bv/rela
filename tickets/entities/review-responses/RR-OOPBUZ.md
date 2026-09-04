---
id: RR-OOPBUZ
type: review-response
title: Comment body is unbounded and unvalidated; no size or rate limit
finding: 'The plan''s input-validation section covers anchor_ref, target ids and author, but says of the body only that it is ''markdown, rendered through the SPA''s existing renderMarkdown -> DOMPurify path''. That addresses XSS on the read side and nothing else. Missing: (1) No maximum body length. An authenticated principal with comment:add can write an arbitrarily large body, and on the file backend that means an unbounded file that must be fully read on every subsequent list. rela caps comparable surfaces elsewhere (cmdexec output caps, attachment max). (2) No cap on comments per target — related to RR-1PCQ42''s cap question but distinct: that one is about disclosure, this is about resource exhaustion. (3) Control characters/NUL in the body are unspecified; the file backend serialises to YAML/TOML where a NUL is at best lossy. Specify an allowlist-shaped validation (max length, reject control chars except newline/tab) at the handler, since the handler is the trust boundary.'
severity: significant
resolution: 'Plan now has an Input validation section, allowlist-shaped and enforced at the handler (the trust boundary): body non-empty with a max length bounded like cmdexec''s output cap; control characters rejected except newline/tab (the file backend serialises to YAML/TOML where NUL is lossy); a per-target comment cap bounding both file size and list cost; and target path segments validated with isSafePathSegment before reaching the file backend. AC12 pins the body rejections.'
status: addressed
---
