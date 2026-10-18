---
id: RR-NOGEJJ
type: review-response
title: A non-promoting action uses two writes where one CAS would do
finding: C3 step 10 goes none to pending to done for a pure record such as acknowledge. SetAction(none to done) is one write, one rev bump and one frame, with the same exactly-once guarantee.
severity: nit
resolution: A non-promoting action is one SetAction none to done CAS.
status: addressed
---
