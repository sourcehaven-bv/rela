---
id: RR-LCG8J1
type: review-response
title: Conformance suite reaches the resolver through a runtime type assertion
finding: visibilitytest.getOne type-asserted r.(resolving) and failed at runtime, the back-channel pattern CLAUDE.md forbids.
severity: significant
resolution: Added visibilitytest.ResolvingReader (Reader plus Resolver()); ReaderMaker returns it and getOne takes it, so the compiler enforces the dependency.
status: addressed
---
