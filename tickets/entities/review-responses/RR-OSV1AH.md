---
id: RR-OSV1AH
type: review-response
title: 'Triggers skipped by in: are visible only at debug level'
finding: 'trigger.go:1239-1242 counts the skip and logs at Debug, copying the ErrNotOpen case. A closed channel is an obvious, self-explaining reason. An in: refusal usually means the operator''s in: is too narrow, or a trigger fires in a state they did not foresee. Example: the PoC ready trigger posts go (in: [new]) after a rework back to queued, and is now silently dropped, where before it posted a plain message. The change is also marked seen (sh.apply), so it never retries. Nothing at info or warn level, and no load-time hint, tells the operator. Fix: log at Info (or Warn once per rule per loaded configuration, like WarnGuardOnce) with the rule, the type and the state. Or document in the trigger section of docs/realtime.md that rep.Skipped / rela scheduler output is where to look.'
severity: minor
resolution: 'A trigger skipped by in: now logs at Warn with the rule, message type and in: key; the docs say so, and TestSessionTriggerPost_MessageIn asserts the log via logtest.Capture.'
status: addressed
---
