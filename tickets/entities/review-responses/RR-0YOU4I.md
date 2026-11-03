---
id: RR-0YOU4I
type: review-response
title: Panel visibility decided from raw config in SPA
finding: The SPA would duplicate server view selection against a wrong TS ViewConfig type; the server should compute the panel boolean.
severity: significant
resolution: The server computes hideMentionedByPanel; the SPA reads only that boolean. TestViewMentions_PanelSwitch and the EntityDetail.mentions frontend test.
status: addressed
---
