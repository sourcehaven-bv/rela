---
id: RR-4UJUN0
type: review-response
title: 'Shutdown''s own stop and the ignored -1 sentinel are untested'
finding: 'Nothing proves the shut-down backend actually stops listening, nor that a listener ignores the -1 broadcast from an older binary during a mixed-version deploy.'
severity: significant
resolution: 'Added TestShutdownClosesListenerConnection (the session is gone after Shutdown) and TestListenerIgnoresShutdownBroadcast (raw pg_notify of -1 then a job is still processed within 5s).'
status: addressed
---
