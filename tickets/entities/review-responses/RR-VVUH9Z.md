---
id: RR-VVUH9Z
type: review-response
title: Test harness could pass vacuously if fixture writes failed
finding: 'scripts/check-tagged-tests-test.sh: make_module and add_test never check their cat redirections. On a full /tmp every module would be empty, every want_tags "" assertion would pass vacuously, and the suite would report all-green having tested nothing.'
severity: minor
resolution: Wont-fix for now, deliberately. The suite's decisive cases are POSITIVE assertions (a specific tag is discovered, a rotted file exits 1); those fail, not pass, if the fixture is empty, so a broken /tmp surfaces as a red suite rather than a vacuous green. Adding error checks to every heredoc would add noise for a failure mode the suite already detects.
reason: 'The failure mode the finding describes is already detected. The suite''s decisive cases are POSITIVE assertions — a named tag must be discovered, a rotted file must exit 1 — and every one of those FAILS when the fixture is empty. A broken /tmp therefore surfaces as a red suite, not a vacuous green. Only the `want_tags ""` cases would pass vacuously, and they are a minority that never stand alone. Adding an error check to every heredoc would add noise proportional to the fixture count for a scenario the suite already catches.'
status: wont-fix
---
