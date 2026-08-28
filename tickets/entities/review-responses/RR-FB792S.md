---
id: RR-FB792S
type: review-response
title: 'A per-file stub list cannot fix this class so the global guard was added now'
finding: 'Stub lists are a denylist maintained by whoever last read a stack trace, so they only ever cover the file that happened to lose the race. The first fix stubbed SidePanel in the one named file and left 15 live requests elsewhere.'
severity: significant
resolution: 'src/test/setup.ts now sets axios.defaults.adapter to throw synchronously at request time on any unmocked call - synchronously by design, so the error lands at the call site in the test that caused it rather than as a nondeterministic teardown crash. Note axios uses the Node http adapter under happy-dom, so patching fetch or XMLHttpRequest catches nothing. Verified 21 stray requests to 0 with all 1662 tests still passing.'
status: addressed
---
