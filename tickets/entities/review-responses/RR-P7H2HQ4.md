---
id: RR-P7H2HQ4
type: review-response
title: Scope must stay a raw type match
finding: 'entity: scope is an exact string match and Matches binds the raw ent.Type. Canonicalising for the clause table would change which events fire and what the program sees.'
severity: significant
resolution: 'Plan: scope keeps slices.Contains on the raw type. Only the compiled-clause lookup falls back to the canonical type, and evaluation passes the raw ent.Type. Alias and case-variant events are in the synthetic golden.'
status: addressed
---
