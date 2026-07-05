---
id: RR-QM9CDV
type: review-response
title: Comments falsely claim trace output honors display_property
finding: kong.go and output.go (entityTitle godoc) claimed 'table, trace, and detail output honor display_property' / 'table and trace paths agree', but trace output builds its own node titles in internal/tracer via e.Title() and is NOT wired to the resolver. Misleading comment.
severity: significant
resolution: Corrected both comments to state only the table and detail (WriteEntity) paths are covered, and explicitly note that trace/path builds its own titles in internal/tracer and is tracked separately (TKT-COZN2E). Filed TKT-COZN2E to resolve trace at the output boundary without giving the pure-reader tracer a metamodel dependency.
status: addressed
---
