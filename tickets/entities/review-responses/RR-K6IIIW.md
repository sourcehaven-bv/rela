---
id: RR-K6IIIW
type: review-response
title: 'C3: load-time error unachievable; constructor returns no error and metamodel never validates automations'
finding: 'Plan required a bad condition: to be a load-time error but NewEngineFromMetamodel (engine.go:37) returns no error - which is precisely why engine.go:61 swallows filter parse failures. metamodel.Load never semantically validates m.Automations so there is no hook. internal/metamodel may not import predicate per arch-lint. Files-to-modify omitted the constructor change and the arch-lint edge.'
severity: critical
resolution: 'NewEngineFromMetamodel gains an error return. Verified exactly 2 production callers (appbuild.go:508 inside buildAutomation which already returns error and appbuildtest/fixture.go:321). arch-lint gains automation+validation -> predicatefns. NewEngine-without-metamodel path defined as a construction error when a condition: is present.'
status: addressed
---
