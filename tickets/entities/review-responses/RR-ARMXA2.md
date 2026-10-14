---
id: RR-ARMXA2
type: review-response
title: 'List condition: and ?query_scope= evaluate hidden properties raw'
finding: 'A list''s condition: and ?query_scope= over a property hidden by visible: are still evaluated on raw values.'
severity: significant
reason: 'Both are operator-authored config: the caller cannot change what they compare, so there is no caller-supplied guess to confirm, and the read gate runs first so they only drop rows the reader may already read. This is the accepted, warned channel recorded in listfieldgate.go. A caller-selectable ?query_scope= is covered by the load-time QueryScopeVisibilityConflicts warning: it iterates every compiled scope (default and named) via compiled.Each, and ?query_scope= can only name a declared scope (an undeclared name is errBadQueryScope). Limit: the warning inspects only entity.<attr> reads in the scope predicate.'
status: wont-fix
---
