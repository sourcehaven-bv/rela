---
id: RR-8J85HFO
type: review-response
title: renametype has the same TrimSpace fence bug
finding: replaceYAMLType used TrimSpace on the closing fence, so an indented --- in an earlier block scalar stopped the scan and a type rename silently skipped the file.
severity: significant
resolution: Exported frontmatter.IsClosingDelimiter and used it in replaceYAMLType; added a TestReplaceYAMLType case with an indented --- before type.
status: addressed
---
