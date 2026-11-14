---
id: RR-0TLEFA1
type: review-response
title: Confusing names in one package
finding: Flows/Flow would shadow Compiled.Flows/Flow, branch sits beside Branch, and GuardError sits beside Guard.
severity: minor
resolution: Accessors are Headless() and HeadlessFlow(name). branch becomes headlessBranch. GuardError keeps its name, because callers match it with errors.As; its doc names the permission. Definition() is dropped.
status: addressed
---
