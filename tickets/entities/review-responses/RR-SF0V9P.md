---
id: RR-SF0V9P
type: review-response
title: Causing message of a transition stays editable with no audit trail of the props the guard saw
finding: 'internal/realtime/ops.go:67 refuses edit only for the notice itself, so the author of the message that fired a transition can still edit it afterwards (plan item 11 TestSessionLifecycleAndEdits pins this). The when: guard ran on the original props (internal/realtime/session.go:186-214). channels has no edit audit op (internal/channels/channels.go:578-593), and neither the notice nor the AuditSession record (internal/channels/session.go SessionNotice/SessionAudit) stores those props. Attack path: a poster sends a message whose props satisfy a guard (e.g. message.severity == ''low'' to skip escalation), the session moves, and the poster then edits the props. The trail shows cause=#n pointing at content that no longer satisfies the guard, and nothing records the original values. This is an integrity and non-repudiation gap, not an access bypass. Fix: refuse edit of a message that caused a move (have the store mark the cause, or check for a notice at seq+1 whose cause is that seq) and answer 409 session_notice. Alternatively, copy the causing props (or their hash) into the AuditSession record.'
severity: minor
resolution: Stores mark a message that moved the session (MovedSession); the author's edit and delete return 409 session_message and _actions omits them; moderator removal stays; TestSessionMessageProtected; documented.
status: addressed
---
