---
id: RR-T4CZOS
type: review-response
title: commentEntityId is a pure alias with a history comment
finding: const commentEntityId = servedRef added nothing and its comment was mostly history.
severity: nit
resolution: Removed the alias; call sites use servedRef; a two-line reason sits on loadComments.
status: addressed
---
