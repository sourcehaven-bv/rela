---
id: RR-8HCC6O
type: review-response
title: Body-redaction caveat should point at mentions
finding: The policyreader TODO(body-redaction) does not mention the mention index, which becomes a body oracle once bodies are redactable.
severity: nit
resolution: The policyreader TODO(body-redaction) now says that Resolver.MentionedBy must learn about body redaction too.
status: addressed
---
