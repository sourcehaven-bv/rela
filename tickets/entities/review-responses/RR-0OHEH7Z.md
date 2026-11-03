---
id: RR-0OHEH7Z
type: review-response
title: Drawer start fires before messages are loaded
finding: Starting the decision once the head loads can run onChoose before the question and notice are held, or with hasNewer after a reveal, so findChoice fails and the user wrongly gets the stale toast.
severity: significant
resolution: Start only after the head and first message page arrived and any reveal finished; serve session.cause on the head so decisionCause reads the stored cause; for Other with a matching generation but no built choice, fall back to onAnswer(answerType, cause) without a stale toast. Test cases added to AC 10.
status: addressed
---
