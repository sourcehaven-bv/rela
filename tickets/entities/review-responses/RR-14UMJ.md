---
id: RR-14UMJ
type: review-response
title: url.QueryEscape used in rewriter and Lua binding with asymmetric semantics
finding: internal/dataentry/document.go:433 escapes only the return_to value and re-emits existingQuery verbatim. internal/lua/urls.go:173-175 escapes both keys and values uniformly. Not wrong today (authors aren't adversarial), but inconsistent if a user-supplied query ever feeds rela.url. Document the asymmetry or share a helper.
severity: minor
reason: Asymmetric url.QueryEscape behaviour between the rewriter (escapes only return_to; re-emits author's query verbatim) and the Lua binding (escapes both keys and values uniformly) is correct for today's usage — authors aren't adversarial and the two paths have different inputs. Extracting a shared helper would require deciding a canonical escape strategy for rewriter input, which isn't necessary now. Defer until a real asymmetry-induced bug surfaces.
status: deferred
---
