---
id: RR-NO4VF
type: review-response
title: AnalysisSummary JSON omits ScriptError/LoadError counts
finding: 'AnalysisSummary has ValidationErrors/Warnings (counted from Violations only) but nothing for ScriptErrors/LoadErrors. JSON output for analyze all shows validation_errors: 0 even when 5 rules failed to compile. CI consumers parsing JSON see clean runs while text output shows failures. Location: internal/cli/analyze.go:496-538, internal/workspace/analysis.go:386-418.'
severity: significant
resolution: AnalysisSummary gained ValidationScriptErrors and ValidationLoadErrors fields, populated by AnalyzeAll. The analyze-all JSON envelope now includes validation_script_errors and validation_load_errors keys, and the text summary box shows them when non-zero. New TestAnalyzeAll_JSONIncludesScriptAndLoadErrorCounts verifies the JSON shape and error status. Commit 6aeb12c.
status: addressed
---
