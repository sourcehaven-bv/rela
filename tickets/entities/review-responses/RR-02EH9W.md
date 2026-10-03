---
id: RR-02EH9W
type: review-response
title: Skip helper builds a bare documentService to reach one method
finding: requireCmdexecSandbox constructs documentService{} only to call executeCommand; call cmdexec directly.
severity: nit
resolution: requireCmdexecSandbox calls cmdexec.New and Runner.Run directly.
status: addressed
---
