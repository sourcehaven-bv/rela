---
id: RR-QQXTTI
type: review-response
title: sessionAdmits reverse-engineers its own closure with errors.AsType
finding: 'internal/realtime/session.go:341-367: admitIn builds a func(Channel) error, and sessionAdmits then calls admitIn again, invokes the closure and type-asserts the result back to *SessionStateError. The typed check is the primitive and the closure is the adapter, but the code builds it the other way round. Simpler: func sessionRefusal(s, mt, ch) *channels.SessionStateError holds the logic, and admitIn returns func(ch) error { if e := sessionRefusal(s, mt, ch); e != nil { return e }; return nil } (the explicit nil return avoids the typed-nil trap). That removes one function, the AsType round trip and the closure allocation per type per postable() call. The plan''s c.sessionAdmits(mt) method would also remove the repeated sessionAdmits(c.ct.Session(), mt, sessionChannel(c.head, c.now)) at service.go:298/389.'
severity: nit
resolution: sessionRefusal(s, mt, ch) is now the typed primitive and admitIn adapts it, with no AsType round trip; it stays a free function because a call method would put call over the plimsoll cap of 40.
status: addressed
---
