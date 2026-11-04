---
id: RR-JSNHRW0
type: review-response
title: Unknown-view log carries unbounded client text
finding: '[security] parseNextActionPage logged the client-supplied view at Warn with no length cap. A caller could fill logs at will with arbitrary text.'
severity: nit
resolution: 'Logged at Debug: the cause is expected and the text is client-controlled.'
status: addressed
---
