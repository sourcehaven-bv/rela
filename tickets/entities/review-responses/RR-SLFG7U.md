---
id: RR-SLFG7U
type: review-response
title: Sidebar keeps entity titles after a refused refetch
finding: SidebarEntityQuery built rows from query.data, which Pinia Colada keeps across an error, and never consulted shouldDropHeldContent. After a 401 (expired assertion) or other refusal the sidebar kept showing titles on every route. The comment claiming parity with EntityList was also wrong.
severity: significant
resolution: A denied computed (status error and shouldDropHeldContent) empties rows and overflow and sets failed; transient failures keep the held rows. Paired tests for 401/403/404 and 500 added and mutation-checked. Comment corrected.
status: addressed
---
