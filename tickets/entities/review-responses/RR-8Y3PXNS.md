---
id: RR-8Y3PXNS
type: review-response
title: Dialog create retry makes duplicates
finding: In LLMProposalDialog a create whose relation failed stayed ticked, so Apply again created a second entity.
severity: significant
resolution: The dialog deletes the new entity when its relation fails; if that also fails it settles the row and names the kept id. Tests cover both.
status: addressed
---
