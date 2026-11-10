---
id: RR-DB7TC6V
type: review-response
title: readable error branch is unreachable with ScriptReader
finding: ScriptReader answers gate failures as ErrNotFound, so the IssueFailed branch fires only with other implementations.
severity: minor
resolution: Documented on ReadGate; the branch stays for other implementations and fails closed.
status: addressed
---
