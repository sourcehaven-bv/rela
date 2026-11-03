---
id: RR-MP7MZT
type: review-response
title: messageInCheck is a three-field struct used as a namespace for two one-shot calls
finding: 'internal/realtimecfg/message_in.go:14-105: withoutSession uses only v and ct.Name. withSession and one pass s, rawStates, raw and null as parameters anyway, and one takes five arguments while also reading m.v and m.ct. It follows the waitChecker/writeChecker precedent, so it is defensible. But two free functions, messageInWithoutSession(v, raw, ct) and messageIn(v, raw, ct, s, rawStates), would be shorter with no loss. If the struct stays, let one look up raw and null from mn itself instead of taking them as parameters.'
severity: nit
resolution: messageInCheck stays (waitChecker precedent, keeps validator under plimsoll), but one() now looks up raw and null from mn itself and takes only mn, s and rawStates.
status: addressed
---
