---
id: RR-CY4X9
type: review-response
title: demo-schema-render.sh does not parse-check DOT and has no hyphenated entity
finding: 'The `check` function uses `grep -qE` against the DOT text — purely lexical. It does not run `dot -Tdot` as a parse check. Combined with no hyphenated entity in the fixture, this lets the critical hyphenation bug slip through. Fix: add a hyphenated entity to the demo''s metamodel, and pipe DOT through `dot -Tdot -o /dev/null` as an explicit parse-validity check before the PNG render.'
severity: significant
resolution: 'demo-schema-render.sh now (a) includes `review-response` as a hyphenated entity type with a `responds-to` relation, (b) pipes both the main DOT and the --exclude DOT through `dot -Tdot` as a pure parse check before any render. Verified locally: demo runs green with all assertions passing and both parse checks succeed.'
status: addressed
---
