---
id: RR-IQOM1O
type: review-response
title: Markdown to HTML silently drops content
finding: Tables, images, raw HTML and relative links were lost without telling the connector, so a push could degrade both copies.
severity: significant
resolution: Tables are supported end to end; both functions return a lossy flag and the docs tell connectors not to push a lossy body.
status: addressed
---
