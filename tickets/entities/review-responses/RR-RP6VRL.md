---
id: RR-RP6VRL
type: review-response
title: DefaultInKeyed is exported but used only inside realtimecfg
finding: 'internal/realtimecfg/message_in.go:33: nothing outside the package reads DefaultInKeyed, and by design nothing reads default_in after the load (Session has no field). OnFinalClose is exported because internal/realtime/outputs.go:341 reads it; this constant has no such reader. Fix: unexport it as defaultInKeyed and update the [DefaultInKeyed] doc link in raw.go:58, or the doclink gate breaks.'
severity: nit
resolution: Unexported it as defaultInKeyed; the raw.go doc comment now says "keyed or empty" instead of the link.
status: addressed
---
