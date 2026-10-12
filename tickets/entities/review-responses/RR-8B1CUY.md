---
id: RR-8B1CUY
type: review-response
title: Section index alignment is guarded by a comment and one test
finding: resolveSectionCreate pairs built sections with the config by index and skips silently when i >= len(sections). A future caller that omits first would get wrong buttons rather than a crash.
severity: minor
reason: TestHideEmpty_KeepsCreateOnLaterSection fails if the omit moves above resolveSectionCreate. Restructuring build and resolve into one helper touches the side-panel path too and belongs with FEAT-KQ45P, which unifies the detail screen.
status: deferred
---
