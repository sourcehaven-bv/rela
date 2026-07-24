---
id: RR-PBTUK5
type: review-response
title: arch-lint component edits required for cmdexec and transform
finding: .go-arch-lint.yml requires every internal package to be a declared component with an explicit mayDependOn allow-list. New internal/cmdexec (pure leaf) and internal/transform components must be added; cmdexec added to attachment.mayDependOn and dataentry.mayDependOn; transform must NOT import dataentry (only entry points and cli may).
severity: minor
resolution: 'Plan updated: add cmdexec (empty mayDependOn) and transform (mayDependOn: entity, metamodel, script, dataentryconfig — NOT dataentry) components; add cmdexec to attachment and dataentry mayDependOn. Run just arch-lint as part of implementation. The list renderer living in dataentry (RR-A) keeps transform free of a dataentry import.'
status: addressed
---
