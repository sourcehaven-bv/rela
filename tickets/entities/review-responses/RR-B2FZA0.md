---
id: RR-B2FZA0
type: review-response
title: loadEntry does not wrap the store error
finding: loadEntry replaces the store error with a fixed not-found message.
severity: nit
reason: Render error text reaches the client in the 500 detail, so wrapping the store error would expose backend error text.
status: wont-fix
---
