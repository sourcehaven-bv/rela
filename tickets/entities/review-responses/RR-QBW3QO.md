---
id: RR-QBW3QO
type: review-response
title: rela db reconcile could derive an index shape the server never derives
finding: staticIndexProps justified skipping a non-compiling condition with 'conditionlint refuses it at load', but queryplan.LoadStaticIndexSpecs (the CLI's `rela db reconcile` path) ran only dataentryconfig.ValidateConfig, never conditionlint. A broken condition would converge to the query-only index while the server refuses the config.
severity: minor
resolution: 'LoadStaticIndexSpecs runs conditionlint.CompileNextActions and returns an error on any problem (arch-lint: queryplan may depend on conditionlint); TestLoadStaticIndexSpecsRejectsBrokenCondition. staticIndexProps''s comment narrowed to the in-code Config case.'
status: addressed
---
