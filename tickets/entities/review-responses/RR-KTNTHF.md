---
id: RR-KTNTHF
type: review-response
title: A busy type with no working state loads silently
finding: 'internal/dataentryconfig/channels.go:239-257 refuses a type without a session because "it is never busy", but accepts a session whose states are all waiting_for or final (as the `idle` type in internal/realtime/busy_test.go:97-106). That field also never shows anything, and Busy returns ok=false for it at runtime. Fix: add `HasWorkingState bool` to BusyFacts (set it from `len(sess.WorkingStates()) > 0` in internal/appbuild/realtime_channels.go:74-90) and refuse, or at least warn, when it is false. Add a case to TestValidateChannels_Busy.'
severity: minor
resolution: Added BusyFacts.HasWorkingState, set from the session's working states in appbuild; busyFieldErrors refuses a busy type whose session has no working state; tests in dataentryconfig and appbuild; guides updated.
status: addressed
---
