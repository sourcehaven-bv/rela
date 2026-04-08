---
id: RR-B6A96
type: review-response
title: TestMain swallows os.Setenv errors
finding: The TestMain added for this refactor swallowed os.Setenv errors with `_ = os.Setenv(...)`. If the process environment becomes read-only (rare, but possible in some CI sandboxes or seccomp-restricted containers), the tests would silently skip the setup and all leak tests would run with the env vars unset, defeating the purpose of the sentinel-key assertion. TestMain is the one place where a panic is the right move because the whole suite is bogus without the setup.
severity: nit
resolution: Rewrote TestMain to loop over the env var names, call os.Setenv in each iteration, and panic on any error via `panic(fmt.Sprintf(...))`. Comment updated to explain why panic is the right failure mode here.
status: addressed
---
