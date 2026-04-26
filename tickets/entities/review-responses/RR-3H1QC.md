---
id: RR-3H1QC
type: review-response
title: rela analyze validations exits 0 on script failure
finding: 'runValidations returns nil regardless of result.HasErrors(). CI piping rela analyze validations won''t fail when Lua compile errors exist — opposite of AC1''s promise. rela validate --check validations correctly bubbles hasErrors to non-zero exit; analyze doesn''t. Inconsistent. Location: internal/cli/analyze.go:374-466.'
severity: significant
resolution: runValidations now returns errors.NewExitError(1) when error-severity violations exist or any rule fails to run (ScriptError/LoadError), in both text and JSON paths. Mirrors the rela validate --check validations behaviour. New TestAnalyzeValidations_NonZeroExitOnScriptError verifies the exit code is 1 when a Lua compile error occurs. Commit 6aeb12c.
status: addressed
---
