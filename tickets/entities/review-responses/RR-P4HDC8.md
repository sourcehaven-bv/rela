---
id: RR-P4HDC8
type: review-response
title: Notices name roles by machine name while the stepper uses labels, and old notices use the current config
finding: 'The schema descriptor''s state waiting_for carries role names only (internal/dataentry/channels_schema.go:62, channelView.ts:254-256), so notices read "waiting for lead" while the stepper reads "Waiting for the incident lead". Two texts in one panel disagree. noticeView also looks up waiting_for in the current descriptor, so every historical notice in the log gains "waiting for ..." from today''s realtime.yaml, which is wrong after a reload and suggests a wait that has long ended. Fix: serve role labels in the descriptor (config, not secret), for example waiting_for as [{role,label}] or a roles map, and use them in notices. Add waiting_for only to the newest notice or the live announcement, not to every logged notice.'
severity: minor
resolution: Notices no longer name waiting roles since current config cannot say whom an old state waited for; the stepper waiting line is a polite live region; docs updated
status: addressed
---
