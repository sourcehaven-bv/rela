---
id: RR-HF2BZ2
type: review-response
title: MessagePresentation.Display is a plain string
finding: 'channels.go:38 declares Display as string, with two untyped string constants. A named type `type MessageDisplay string`, with typed constants and the valid-value slice, documents the domain and keeps a stray string out at compile time. It also matches how the widget values are handled. Fix: add the named type; the yaml and json tags are unchanged.'
severity: nit
resolution: Fixed. Added type MessageDisplay string with typed constants and a typed valid-value slice; yaml and json tags unchanged.
status: addressed
---
