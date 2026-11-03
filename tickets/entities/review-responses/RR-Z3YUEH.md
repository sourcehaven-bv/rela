---
id: RR-Z3YUEH
type: review-response
title: SessionStep.Admit contract promises "returned unchanged" but only sql-backend passthrough of ErrSessionState makes that true
finding: 'internal/channels/session.go:276-281 documents that a non-nil error from Admit is returned by the store unchanged, for any error. internal/channels/sqlchannels/sql.go:375 passes an error through unwrapped only when it matches a sentinel in its list, so any other error Admit returns gets wrapped as "<driver>: ...: %w" on pg/sqlite and stays bare on filechannels. The conformance case (channelstest/session.go testSessionAdmit, require.Same) passes only because it returns a *SessionStateError. The generic hook therefore has a narrower contract than its doc. Fix: either state that Admit must return an error wrapping ErrSessionState (and say so at channels.go:125 too), or reword the doc to "an error wrapping ErrSessionState is returned unchanged" so it matches what every backend does. Do not leave a hook that looks generic but only works for one error type.'
severity: minor
resolution: The Admit and ErrSessionState docs now say Admit returns nil or an error wrapping ErrSessionState, which every backend returns unchanged, and that any other error may be wrapped.
status: addressed
---
