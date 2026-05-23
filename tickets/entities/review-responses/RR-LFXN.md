---
id: RR-LFXN
type: review-response
title: Whitespace-only title leaks through `||` fallback
finding: '`issue.title || issue.entityId` treats a whitespace-only title (e.g. "   ") as truthy and renders blank space on the title line instead of falling back to the entityId. Backend `DisplayTitle` does not trim, so a metamodel `primary_property` pointing at a whitespace-only string could surface this. Low probability but easy fix: `return issue.title?.trim() || issue.entityId`.'
severity: minor
resolution: 'Added `.trim()` to the fallback: `return issue.title?.trim() || issue.entityId`. Added a vitest case `falls back to the entityId on the title line when title is whitespace-only` covering the `''   ''` title input.'
status: addressed
---
