---
id: RR-ZRP4R
type: review-response
title: extractProperties helpers are methods but use no receiver state
finding: extractProperties and extractPropertiesAllowNil are on *Server but use no Server state. Could be free functions like parsePropertiesArg. Minor consistency issue.
severity: nit
resolution: Converted extractProperties and extractPropertiesAllowNil to free functions (matching parsePropertiesArg). Updated the three call sites in tools_entity.go and tools_relation.go, and the existing tests in convert_test.go. validatePropertyNames stays a method because it uses s.ws.Meta().
status: addressed
---
