---
id: RR-ENBEFKL
type: review-response
title: Trace an identity-less inbox request instead of accepting
finding: Add requests without identity so archplan sees a refusal.
severity: minor
reason: Tried it. Identity-less requests get 401 at the request edge before the handler runs, so the trace never reaches handleV1Inbox and the finding stays. Removed again.
status: wont-fix
---
