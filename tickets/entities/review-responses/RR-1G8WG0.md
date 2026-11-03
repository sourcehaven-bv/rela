---
id: RR-1G8WG0
type: review-response
title: Share the YAML 1.1 bool predicate through a leaf package instead of a third copy
finding: isYAML11Bool plus the quoted-style test already exists twice in dataentryconfig (config.go:351-368, dashboard.go:71-75). A third copy in realtimecfg is a fact stored three times. The confirm semantics do differ (here false is meaningful and an empty text is refused; in data-entry.yaml an empty string means no confirm, config.go:358), so do not share the union, only the scalar rule. Add a leaf, for example internal/yamlscalar with PlainYAML11Bool(n *yaml.Node) bool, list it in .go-arch-lint.yml with no dependencies, and add it to realtimecfg's and dataentryconfig's mayDependOn (realtimecfg at .go-arch-lint.yml:1456-1467). That is about 10 lines plus an arch-lint entry, and replaces the "pinned by tests on the same inputs" mitigation, which cannot cover "" anyway because the two packages answer it differently. Also refuse a plain block scalar (| or >) whose value is yes or no the same way, which the shared predicate gets for free.
severity: minor
resolution: Added leaf internal/yamlscalar with PlainYAML11Bool, a component entry in .go-arch-lint.yml and mayDependOn edges from realtimecfg and dataentryconfig. Both dataentryconfig call sites switch to it, which is cheap and safe; the predicate trims, so a | block holding yes is now refused too.
status: addressed
---
