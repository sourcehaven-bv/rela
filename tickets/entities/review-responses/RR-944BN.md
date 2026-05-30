---
id: RR-944BN
type: review-response
title: Static map blocks per-test widget replacement
finding: Static module-scope Record<string, WidgetEntry> means tests can not swap widgets without module monkey-patching, can not register stubs per-suite, can not have parallel registries. The 'plugin-style widgets' future need cited by author is already here -- it is called testing.
severity: significant
resolution: 'Plan revised: defineWidgetRegistry() factory returning {register, resolve}. defaultRegistry is the production singleton. Tests construct isolated registries. See TKT-MZSIJ ''Registry shape''.'
status: addressed
---
