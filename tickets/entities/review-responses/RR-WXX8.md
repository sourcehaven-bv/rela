---
id: RR-WXX8
type: review-response
title: Variable substitution not applied per-token in in/ne operators
finding: in and ne split on comma after substitution, so $today,$tomorrow as a value won't resolve either side. Either substitute per token or document the limit.
severity: significant
resolution: Added resolveFilterVariablesInList that splits on comma, resolves each token, and rejoins. The handler dispatches to the list resolver for in/ne operators and the single-value resolver for the rest. Test TestV1FilteringInWithVariableTokens verifies $yesterday,$today resolves both.
status: addressed
---
