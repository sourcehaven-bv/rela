---
id: RR-IF2Z8I3
type: review-response
title: Typo next to a valid builtins field is accepted
finding: 'search: {hidden: true, permision: x} loads because yaml.v3 ignores unknown nested keys.'
severity: nit
reason: data-entry.yaml checks unknown keys only at the top level; every nested block has the same behaviour. A typo that leaves an entry empty is refused. Strict nested keys belong in a config-wide change.
status: wont-fix
---
