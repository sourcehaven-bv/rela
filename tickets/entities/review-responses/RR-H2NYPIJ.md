---
id: RR-H2NYPIJ
type: review-response
title: Lockfile moves transitive packages, including obug 2 to 3
finding: Besides the vitest packages the lockfile moves chai 6.3.0, obug 3.0.0 (a major bump), tinybench 6.2.2, tinyrainbow 3.2.0 and magic-string 1.4.5. Confirm that vitest 5.0.5 requires these.
severity: minor
resolution: 'Confirmed in the lockfile: vitest 5.0.5 and @vitest/coverage-v8 5.0.5 declare obug ^3.0.0, and the other moves are within the ranges they declare. Both test suites pass on the new tree.'
status: addressed
---
