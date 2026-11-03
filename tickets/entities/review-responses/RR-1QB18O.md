---
id: RR-1QB18O
type: review-response
title: Same log-capture restore bug in about 25 tests outside appbuild
finding: dataentry, mail, lease, kvlease, acl, visibility, secrets, autocascade, cli, mcp, lua, pgstore and affordances tests swap slog.SetDefault and restore it without restoring the log package output.
severity: significant
resolution: The helper moved to the stdlib-only package internal/testutil/logtest (so secrets can import it) and every test that swapped the default logger uses logtest.SetDefault or logtest.Capture. A forbidigo rule bans slog.SetDefault in tests outside logtest.
status: addressed
---
