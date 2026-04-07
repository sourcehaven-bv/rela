---
id: RR-SDCY
type: review-response
title: 'L4: NewRouter tolerates a nil security config — easy to ship without hardening'
finding: 'App.security is nil unless SetSecurityConfig has been called. This is a footgun: production code that forgets to call SetSecurityConfig silently runs without hardening.'
severity: nit
reason: The nil-disabled mode is currently used by every test in the existing dataentry package (router_test.go, e2e_test.go, watcher_test.go, etc.) and by the Wails desktop app (cmd/rela-desktop/main.go) which has a different IPC boundary. Forcing all of those to declare a security config touches dozens of test fixtures. The production entry point (cmd/rela-server/main.go) DOES set the config and log.Fatals on error, which is the only deployment that ships to end users. Worth doing as a follow-up cleanup but not a security blocker.
status: deferred
---
