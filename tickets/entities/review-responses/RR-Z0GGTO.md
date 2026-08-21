---
id: RR-Z0GGTO
type: review-response
title: 'Don''t accept widget-table drift: the two resolvers already disagree on `file`'
finding: 'PLAN-DMQFRJ Risk section proposed duplicating the widget/property-type compatibility table in Go, mirroring supportedPropertyTypes (registry.ts:105-117), and ACCEPTING the drift risk as disproportionate to solve for ten entries. That arithmetic was wrong because it assumed only two copies. Per RR-YTC4W5 a third already exists (Metamodel.ResolveWidgetFromType, schema_output.go:117), and the existing two have ALREADY drifted: ResolveWidgetFromType has no `file` case so a file property resolves to ''text'', while defaultWidgetFor (registry.ts:26) returns ''file''. It also lacks list/values handling, which registry.ts:19-20 places ABOVE the scalar switch and documents as load-bearing order (RR-0Z1P6). Critically, the plan''s own suggestion to ''derive the Go table from the same list'' via ResolveWidgetFromType would import the file bug and REJECT `widget: file` on a file property -- which the plan''s own Edge Cases calls legal.'
severity: significant
resolution: 'Verified the divergence: grep confirms no file case in schema_output.go (falls through default -> ''text'') vs registry.ts:26 returning ''file''. Adopted reviewer option 1: build the Go compatibility table as its own map literal, NOT derived from ResolveWidgetFromType, plus a two-sided fixture test -- a Vitest test asserting registry supportedPropertyTypes matches a shared fixture, and a Go test asserting the same fixture. Roughly the cost of the comment the plan originally proposed, but it fails CI on drift instead of documenting it.'
status: addressed
---
