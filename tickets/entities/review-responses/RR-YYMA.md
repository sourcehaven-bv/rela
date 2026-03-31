---
id: RR-YYMA
type: review-response
title: Quiet flag not respected in JSON output
finding: JSON output branches always emit output regardless of quiet flag.
severity: significant
reason: JSON output is specifically requested via -o json and represents validation results. Suppressing with --quiet would leave no output. The --quiet flag is for progress messages, not results.
status: wont-fix
---
