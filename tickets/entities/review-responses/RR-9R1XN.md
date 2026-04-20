---
id: RR-9R1XN
type: review-response
title: 'Critical: userstate not declared in .go-arch-lint.yml — arch-lint CI job would fail on every PR'
finding: 'cranky-code-reviewer #2: userstate was added as a package but not registered as a component in .go-arch-lint.yml. 18 component violations would trigger the arch-lint job. Nobody ran ''just ci'' locally before pushing.'
severity: critical
resolution: Added userstate component + dependency declarations (cli, cmdServer, cmdDesktop, app, workspace, encryption all mayDependOn userstate). Added lockedfile vendor. Verified with `~/go/bin/go-arch-lint check` — OK, no warnings. Also added userstate's own mayDependOn rules for project, state, storage.
status: addressed
---
