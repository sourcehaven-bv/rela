---
id: RR-LM249M
type: review-response
title: anySystemIdentity is unexported in internal/scheduler
finding: Section 4 calls ValidateRunAs(v, anySystemIdentity), but that function is private to the scheduler package. The CLI needs its own allow function, which decision 3's restriction would change anyway.
severity: nit
resolution: The CLI has its own allow function for the restricted names.
status: addressed
---
