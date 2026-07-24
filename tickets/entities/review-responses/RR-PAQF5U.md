---
id: RR-PAQF5U
type: review-response
title: Metamodel guide doesn't list `description` as a root-only include field
finding: 'GUIDE-metamodel.md (regenerated into docs/metamodel.md) says included partials ''must not contain version: or namespace:''. The code now also rejects description: in includes (include.go:137) but the doc sentence wasn''t updated — doc/code drift. One-word fix: add ''or description:''.'
severity: minor
resolution: Updated the include-file root-only sentence in GUIDE-metamodel.md (source) to include `description:` alongside version/namespace, and regenerated docs/metamodel.md. Doc now matches the code.
status: addressed
---
