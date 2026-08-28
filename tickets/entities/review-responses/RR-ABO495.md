---
id: RR-ABO495
type: review-response
title: 'Navigation docs table documented a nonexistent graph: field'
finding: 'The navigation field table documented `graph: bool` ("Link to the graph explorer"), which does not exist on NavigationEntry, while the real `search:` and `settings:` fields were undocumented. A YAML example also used `graph: true`, and the counts paragraph referred to "graph entries". Pre-existing, but in the table this ticket edits.'
severity: nit
resolution: 'Replaced the graph row with the real search/settings rows, changed the example entry to `search: true`, and reworded the intro and counts paragraphs to name the kinds that actually exist. Fixed in the SOURCE guide (docs-project/entities/guides/) and regenerated — docs/*.md are generated output.'
status: addressed
---
