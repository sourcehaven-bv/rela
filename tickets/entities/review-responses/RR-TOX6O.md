---
id: RR-TOX6O
type: review-response
title: CLI subcommand inline dispatch brittle past one subcommand
finding: cmd/rela-server/main.go:44-47 dispatches via os.Args[1] == "routes". Fine for one subcommand; accretes painfully. Plan already acknowledged a cobra migration when the second lands. Defer.
severity: nit
reason: Inline os.Args dispatch is fine for one subcommand; cobra migration was explicitly called out in the plan as a follow-on when the second subcommand lands. No action this ticket.
status: deferred
---
