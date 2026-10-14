---
id: RR-MJ8XRF
type: review-response
title: Query scopes compare raw values of hidden properties
finding: A query scope's props and filter evaluate stored values, so a config-fixed scope over a hidden property leaks one bit per row. Gate it or record it as an accepted membership channel.
severity: minor
reason: 'A query scope is operator config, not a client query. Its comparison on a hidden property is the accepted, warned one-bit membership channel: appbuild QueryScopeVisibilityConflicts warns at load. Out of scope for this bug.'
status: wont-fix
---
