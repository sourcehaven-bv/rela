---
id: RR-E2VU0MO
type: review-response
title: rela.search bridge could not page
finding: The app bridge search call had no page parameter, so an app could not read past the first page.
severity: minor
resolution: rela.search accepts page (positive integer, else invalid_params) and passes it through; bridge test added.
status: addressed
---
