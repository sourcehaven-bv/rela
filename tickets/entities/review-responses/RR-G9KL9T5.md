---
id: RR-G9KL9T5
type: review-response
title: Concurrent organization.updated events can resume a suspended tenant
finding: 'Two organization.updated events for one org could race: one reads active, the other reads suspended and suspends, then the first resumes the tenant.'
severity: significant
resolution: 'Fixed in f71a306bb: syncs of one org take turns in each process and read the IdP only once they hold the turn. Across processes, a resume that changed the status reads the IdP again and suspends again if needed. Pinned by TestStatusSync_ConcurrentSyncsTakeTurns and TestStatusSync_ResumeRechecksTheIdP.'
status: addressed
---
