---
id: RR-VYTL35
type: review-response
title: Validate transform 'produces' as a well-formed MIME type at load
finding: The transform 'produces' string is echoed into the HTTP Content-Type response header. Though operator-config trust, an unvalidated value could carry CRLF (header injection) or be malformed.
severity: significant
resolution: validateTransforms parses 'produces' with mime.ParseMediaType at metamodel load; reject CRLF/control chars and malformed media types. Whole metamodel load fails on a bad entry so a broken registry can never half-work at request time.
status: addressed
---
