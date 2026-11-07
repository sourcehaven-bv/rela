---
id: RR-TI5T9S8
type: review-response
title: CI dependency-isolation checks cannot fail
finding: set -e ignores pipelines negated with !; every ! go list | grep -q line in Assert dependency isolation was a no-op (security reviewer found the same).
severity: significant
resolution: Added a forbid helper that captures go list output and fails explicitly; converted all negated lines; conflicting-tag step uses if/then exit 1. Ran both steps locally and confirmed forbid fails on a real match.
status: addressed
---
