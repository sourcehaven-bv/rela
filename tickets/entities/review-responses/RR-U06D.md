---
id: RR-U06D
type: review-response
title: appbuild.go wiring relies on store.Store embedding GraphQueryer
finding: '1-line wiring works because store.Store embeds store.GraphQueryer. Fine for now but a future build tag that wraps store.Store in a decorator (audit, metrics) must forward GraphQueryer or this compiles while the gate silently uses the wrong store. Fix: one-line comment near the call documenting the constraint.'
severity: nit
resolution: buildACL in appbuild.go gained a comment documenting that st is passed twice (Graph adapter + GraphQueryer) and that store-wrapping decorators MUST forward both.
status: addressed
---
