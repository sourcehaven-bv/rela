---
id: RR-CC6VEW
type: review-response
title: go-mail rejects CRLF in addresses but NOT in Subject — header-injection validation must be ours, not delegated
finding: 'The plan lists CR/LF rejection for addresses and subjects together, implying the library covers both. Verified against go-mail v0.8.1: From()/To() return a parse error on embedded CRLF, but Subject("Hi\r\nBcc: evil@x.com") succeeds and is only neutralized incidentally by RFC 2047 encoded-word escaping (rendering as =?UTF-8?q?Hi=0D=0ABcc:...?=). That is an encoding side effect, not a validation guarantee, and it is not something to depend on across library versions or for header values the library does not encode.'
severity: significant
resolution: Plan now states that header-injection validation is rela's responsibility at enqueue, applied uniformly to every caller-supplied header value (subject and any future custom headers), independent of what the transport library happens to do. Address-level CRLF rejection by go-mail is treated as defence in depth, not the control. Added an explicit negative test asserting a CRLF subject is rejected at enqueue rather than reaching the transport.
status: addressed
---
