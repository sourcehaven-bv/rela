---
id: RR-YGW5
type: review-response
title: Inconsistent output routing
finding: Code mixes fmt.Println with checkOut.WriteSuccess/WriteError. When --output json is specified, fmt.Println calls could corrupt JSON output.
severity: significant
reason: The fmt.Println calls are for progress messages already guarded by quiet checks. JSON output from WriteAnalysisResult is separate. Matches other CLI commands in codebase.
status: deferred
---
