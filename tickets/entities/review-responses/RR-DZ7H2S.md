---
id: RR-DZ7H2S
type: review-response
title: appbuild godoc documented fail-open while its code fails closed
finding: internal/appbuild/appbuild.go luaReadDepsFor's godoc said 'A construction failure is also unrestricted-with-a-warning rather than silent denial' — but the code 30 lines below returns visibility.DenyReader{} and logs slog.Error. The comment documented pre-RR-GKCZO5 behavior and was never updated. Since rela#1198 is an ISO CONTROL-5-15 finding, an auditor reading the file cited as the fail-closed exemplar would find prose asserting the opposite.
severity: significant
resolution: Rewrote the paragraph to state that a construction failure REFUSES via DenyReader/DenyTracer (RR-GKCZO5), pointing at scriptEntityReader for the rationale. The NopACL sentence, which was and remains accurate, is preserved.
status: addressed
---
