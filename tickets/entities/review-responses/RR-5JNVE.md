---
id: RR-5JNVE
type: review-response
title: Stale comments reference removed edit:// / create:// rewriter
finding: internal/script/executor.go:78 ExecuteDocument's doc comment still says the data-entry layer rewrites edit://+create:// links. internal/dataentry/e2e_test.go:286-294 still says the script emits edit:// links via a markdown table. Both are outdated; update to describe the app-relative + return_to flow and note legacy schemes pass through with a warning.
severity: minor
resolution: ExecuteDocument doc comment (internal/script/executor.go:75) updated to describe the app-relative + return_to flow and that legacy schemes pass through with a warning. Also documents that rela.url is only wired in document renders. e2e_test.go:223 comment updated to match current behaviour (rela.url-built /form/edit_ticket/ links plus return_to).
status: addressed
---
