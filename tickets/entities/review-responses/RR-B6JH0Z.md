---
id: RR-B6JH0Z
type: review-response
title: ListColumn godoc still lists two kinds
finding: config.go:956-960 says a column references either a Property or a Relation. With Face and now Busy there are four kinds. Update the type godoc to name all four and that exactly one is set.
severity: nit
resolution: ADDRESSED ListColumn godoc names the four kinds (Property, Relation, Face, Busy) and that exactly one is set.
status: addressed
---
