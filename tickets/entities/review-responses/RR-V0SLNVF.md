---
id: RR-V0SLNVF
type: review-response
title: The open-path guard test missed constructors, pgstore DSN helpers and the URL literal
finding: TestCLIOpensProjectsOnlyThroughOpenProject banned only Discover, At and DiscoverSharedBase.
severity: minor
resolution: It also bans appbuild.New, NewSharedBase, pgstore MigrateDSN/StatusDSN/ReconcileDSN and the RELA_DATABASE_URL literal, with a per-file allowlist and reason.
status: addressed
---
