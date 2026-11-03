---
id: RR-B61FY6
type: review-response
title: Busy entry for a session that never moved carries state "" and a zero since
finding: 'Plan step 2 builds BusyView from the stored row: State is ch.Session.State, Since is the session''s EnteredAt, and the label is looked up from that state. For a session that never moved, the stored session is zero. realtime maps that to the initial state with EnteredAt = ch.CreatedAt (internal/realtime/session.go:126-131, currentSession). The plan does not, so its supported "initial state is working" case (AC 1, Edge cases) would send state "", an empty label (the card''s visible text would be blank) and since 0001-01-01. The plan should derive each entry from currentSession, or from sessionView as inboxRun.item does (internal/realtime/inbox.go:837), keep the entry when the status is working, and add a TestBusy_States case for the never-moved session that asserts the state name, label and since == CreatedAt.'
severity: significant
resolution: Step 2 builds each entry from currentSession after the read gate, dropping stranded and non-working states. AC 2 and TestBusy_States add a never-moved case asserting the initial state name, label and since == CreatedAt.
status: addressed
---
