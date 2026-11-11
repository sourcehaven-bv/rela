---
id: RR-B3BW79V
type: review-response
title: filechannels crash can reuse a seq with stray files
finding: seq derives from the log; a crash after moving files into files/<seq>/ but before the log line lets the next append inherit the stray directory. The file store has no load phase for a sweep.
severity: significant
resolution: Under the lock Append removes files/<seq> before renaming new files in. OpenFile serves only ids listed in Message.Files. Crash test added to the test plan.
status: addressed
---
